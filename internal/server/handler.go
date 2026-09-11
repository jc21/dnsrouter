// Package server ...
package server

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"dnsrouter/internal/config"
	"dnsrouter/internal/logger"

	"github.com/miekg/dns"
	"github.com/patrickmn/go-cache"
)

// defaultCacheCleanupInterval is how often go-cache sweeps expired entries out of memory.
// This is unrelated to how long an individual answer is cached for (see NewCache).
const defaultCacheCleanupInterval = 5 * time.Minute

// udpEDNSBufferSize is advertised to upstream resolvers so they don't have to truncate
// answers larger than the historical 512 byte UDP default (multi-A/CDN answers, DNSSEC
// records, TXT-heavy SPF/DKIM records, etc. commonly exceed that).
const udpEDNSBufferSize = 4096

// udpClient and tcpClient are safe for concurrent use (per github.com/miekg/dns docs), so
// they're created once and shared across every request rather than allocated per-query.
var (
	udpClient = &dns.Client{Net: "udp"}
	tcpClient = &dns.Client{Net: "tcp"}
)

// DNSHandler handles incoming DNS requests for a single configured server/listener.
type DNSHandler struct {
	ServerIndex int
	RouterConf  config.RouterConfig
	// Cache is shared across all configured servers (matching the single, process-wide
	// "cache" section in the config file) and is built once at startup via NewCache -
	// nil means caching is disabled.
	Cache *Cache
}

// Cache is the shared DNS-answer cache. Unlike handing go-cache's Get/Set the raw
// []dns.RR straight through, it deep-copies answers out on read so concurrently-served
// requests can never share (and race on) the same mutable dns.RR objects - dns.Msg.Pack
// mutates each RR's header in place when a reply is written to the wire.
type Cache struct {
	store          *cache.Cache
	minTTL, maxTTL time.Duration
}

// NewCache builds the shared answer cache from configuration, or returns nil if caching
// is disabled. Each entry's TTL is randomized between Min and Max seconds - a real,
// per-entry expiry range, rather than Max merely being an unrelated background
// cleanup-sweep interval as in the previous implementation. Jittering also avoids every
// entry populated around the same time expiring in lockstep.
func NewCache(conf config.CacheConfig) *Cache {
	if conf.Disabled {
		return nil
	}

	minSecs := conf.Min
	if minSecs <= 0 {
		minSecs = 15
	}
	maxSecs := conf.Max
	if maxSecs <= 0 {
		maxSecs = 30
	}
	if maxSecs < minSecs {
		maxSecs = minSecs
	}

	return &Cache{
		store:  cache.New(cache.NoExpiration, defaultCacheCleanupInterval),
		minTTL: time.Duration(minSecs) * time.Second,
		maxTTL: time.Duration(maxSecs) * time.Second,
	}
}

func (c *Cache) get(key string) ([]dns.RR, bool) {
	cached, found := c.store.Get(key)
	if !found {
		return nil, false
	}

	rrs, ok := cached.([]dns.RR)
	if !ok {
		return nil, false
	}
	answer := make([]dns.RR, len(rrs))
	for i, rr := range rrs {
		answer[i] = dns.Copy(rr)
	}
	return answer, true
}

func (c *Cache) set(key string, answer []dns.RR) {
	ttl := c.minTTL
	if c.maxTTL > c.minTTL {
		// #nosec G404 -- jittered cache TTL selection, not security sensitive
		ttl += time.Duration(rand.Int63n(int64(c.maxTTL - c.minTTL)))
	}
	c.store.Set(key, answer, ttl)
}

// ServeDNS will handle incoming dns requests and forward them onwards
func (h *DNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	// A single malformed/unlucky packet (or a future bug) should never be able to take
	// down every listener in the process - the underlying miekg/dns UDP server spawns a
	// bare goroutine per packet with no panic recovery of its own.
	defer func() {
		if rec := recover(); rec != nil {
			logger.Error("HandlerCrashRecovered", fmt.Errorf("query handling failed unexpectedly: %v", rec))
		}
	}()

	if len(r.Question) == 0 {
		return
	}

	msg := dns.Msg{}
	msg.SetReply(r)

	// DNS names are case-insensitive (RFC 1035 §2.3.3), and 0x20-randomization means
	// semantically identical queries can arrive in arbitrary casing. Normalize before
	// matching against the (lower-case) configured regexes, and before using as a cache
	// key, so casing can never cause a rule/cache miss.
	domain := strings.ToLower(msg.Question[0].Name)
	qtype := msg.Question[0].Qtype

	cacheKey := fmt.Sprintf("%d-%s-%d", h.ServerIndex, dns.Fqdn(domain), qtype)

	if answer, found := h.getCached(cacheKey); found {
		msg.Answer = answer
		logger.Debug("[%d] DNSLookup %s %s -> cached", h.ServerIndex, domain, getRecordTypeString(qtype))
	} else if internalAnswer, matched := getDNSAnswerFromInternal(h.RouterConf, msg, domain, h.ServerIndex); matched {
		// The name matched a configured internal record. Treat it as authoritative: even
		// if there's no answer for this specific qtype, don't fall through to the public
		// upstream - that would leak a query for a name explicitly declared internal/
		// private, and would answer a single name inconsistently depending on qtype.
		msg.Answer = internalAnswer
	} else {
		upstreamHost, nxdomain := getDNSServerFromLookup(h.RouterConf, domain)
		logger.Debug("[%d] DNSLookup %s %s -> %s", h.ServerIndex, domain, getRecordTypeString(qtype), upstreamHost)

		if nxdomain {
			msg.SetRcode(r, dns.RcodeNameError)
		} else {
			answer, rcode, err := resolveUpstream(upstreamHost, domain, qtype)
			if err != nil {
				logger.Error("UpstreamError", err)
				return
			}

			if rcode != dns.RcodeSuccess {
				msg.SetRcode(r, rcode)
			} else {
				msg.Answer = answer
				h.setCached(cacheKey, answer)
			}
		}
	}

	if writeErr := w.WriteMsg(&msg); writeErr != nil {
		logger.Error("HandlerWriteError", writeErr)
	}
}

// getCached returns a deep copy of any cached answer for key - see Cache's doc comment
// for why a copy is required.
func (h *DNSHandler) getCached(key string) ([]dns.RR, bool) {
	if h.Cache == nil {
		return nil, false
	}
	return h.Cache.get(key)
}

func (h *DNSHandler) setCached(key string, answer []dns.RR) {
	if h.Cache == nil {
		return
	}
	h.Cache.set(key, answer)
}

// resolveUpstream forwards the query to upstreamHost, advertising an EDNS0 buffer large
// enough to avoid most truncation. If the upstream truncates its UDP answer anyway, the
// query is retried over TCP against the same upstream before giving up.
func resolveUpstream(upstreamHost, domain string, qtype uint16) ([]dns.RR, int, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), qtype)
	m.RecursionDesired = true
	m.SetEdns0(udpEDNSBufferSize, false)

	addr := net.JoinHostPort(upstreamHost, "53")

	resp, _, err := udpClient.Exchange(m, addr)
	if resp != nil && resp.Truncated {
		if tcpResp, _, tcpErr := tcpClient.Exchange(m, addr); tcpErr == nil {
			resp, err = tcpResp, nil
		}
	}
	if resp == nil {
		return nil, 0, err
	}

	return resp.Answer, resp.Rcode, nil
}

// getDNSServerFromLookup returns the upstream DNS server to forward domain to, and
// whether the matched rule says to answer NXDOMAIN instead of forwarding.
func getDNSServerFromLookup(conf config.RouterConfig, domain string) (dnsServer string, nxdomain bool) {
	dnsServer = conf.DefaultUpstream

	for _, upstream := range conf.Upstreams {
		if upstream.CompiledRegex.MatchString(domain) {
			if upstream.NXDomain {
				return "", true
			}
			return upstream.DNSServer, false
		}
	}

	return dnsServer, false
}

func getRecordTypeString(recordType uint16) string {
	switch recordType {
	case dns.TypeA:
		return "A"
	case dns.TypeAAAA:
		return "AAAA"
	case dns.TypeMX:
		return "MX"
	case dns.TypeTXT:
		return "TXT"
	default:
		return fmt.Sprintf("%v", recordType)
	}
}

// getDNSAnswerFromInternal returns the configured internal answer for domain and whether
// an internal record matched at all - distinct from whether it has an answer for the
// requested qtype, so the caller can tell "not an internal name" (fall through to
// upstream) apart from "internal name, but nothing configured for this qtype" (answer
// NODATA, don't leak the query upstream).
func getDNSAnswerFromInternal(conf config.RouterConfig, m dns.Msg, domain string, serverIdx int) (rr []dns.RR, matched bool) {
	rr = make([]dns.RR, 0)

	for _, internalRecord := range conf.InternalRecords {
		if !internalRecord.CompiledRegex.MatchString(domain) {
			continue
		}
		matched = true

		switch m.Question[0].Qtype {
		case dns.TypeA:
			if internalRecord.A != "" {
				if ip := net.ParseIP(internalRecord.A); ip != nil {
					rr = append(rr, &dns.A{Hdr: dns.RR_Header{Name: m.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET}, A: ip})
					logger.Debug("[%d] DNSLookup %s %s -> %s", serverIdx, domain, getRecordTypeString(m.Question[0].Qtype), internalRecord.A)
				}
			}
		case dns.TypeAAAA:
			if internalRecord.AAAA != "" {
				if ip := net.ParseIP(internalRecord.AAAA); ip != nil {
					rr = append(rr, &dns.AAAA{Hdr: dns.RR_Header{Name: m.Question[0].Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET}, AAAA: ip})
					logger.Debug("[%d] DNSLookup %s %s -> %s", serverIdx, domain, getRecordTypeString(m.Question[0].Qtype), internalRecord.AAAA)
				}
			}
		case dns.TypeTXT:
			if internalRecord.TXT != "" {
				rr = append(rr, &dns.TXT{Hdr: dns.RR_Header{Name: m.Question[0].Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET}, Txt: []string{internalRecord.TXT}})
				logger.Debug("[%d] DNSLookup %s %s -> %s", serverIdx, domain, getRecordTypeString(m.Question[0].Qtype), internalRecord.TXT)
			}
		case dns.TypeMX:
			if internalRecord.MX != "" {
				lines := strings.Split(internalRecord.MX, "\n")
				for _, line := range lines {
					if line != "" {
						d := fmt.Sprintf("%s 0 IN MX %s", m.Question[0].Name, line)
						if mx, err := dns.NewRR(d); err == nil {
							rr = append(rr, mx)
						}
					}
				}
				logger.Debug("[%d] DNSLookup %s %s -> %s", serverIdx, domain, getRecordTypeString(m.Question[0].Qtype), internalRecord.MX)
			}
		default:
			// No record type configured for this qtype; matched stays true so the
			// caller answers NODATA rather than forwarding upstream.
		}

		break
	}

	return rr, matched
}
