// Package server ...
package server

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"dnsrouter/internal/config"
	"dnsrouter/internal/logger"

	"github.com/miekg/dns"
	"github.com/patrickmn/go-cache"
)

var (
	once     sync.Once
	memCache *cache.Cache
)

// DNSHandler ...
type DNSHandler struct {
	ServerIndex int
	RouterConf  config.RouterConfig
}

func initMemCache() {
	memCache = cache.New(30*time.Second, 1*time.Minute)
}

// ServeDNS will handle incoming dns requests and forward them onwards
func (h *DNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	once.Do(initMemCache)
	c := new(dns.Client)

	msg := dns.Msg{}
	msg.SetReply(r)

	domain := msg.Question[0].Name

	// See if we have this cached
	cacheKey := fmt.Sprintf("%d-%s-%d", h.ServerIndex, dns.Fqdn(domain), msg.Question[0].Qtype)
	cacheItem, found := memCache.Get(cacheKey)

	if found {
		// Using our cached answer
		msg.Answer = cacheItem.([]dns.RR)
		logger.Debug("[%d] DNSLookup %s %s -> cached", h.ServerIndex, domain, getRecordTypeString(msg.Question[0].Qtype))
	} else {
		// Look up from internal first
		internalAnswer := getDNSAnswerFromInternal(h.RouterConf, msg, domain, h.ServerIndex)
		if len(internalAnswer) > 0 {
			msg.Answer = internalAnswer
		} else {
			// use upstream next
			upstreamHost := getDNSServerFromLookup(h.RouterConf, domain)
			logger.Debug("[%d] DNSLookup %s %s -> %s", h.ServerIndex, domain, getRecordTypeString(msg.Question[0].Qtype), upstreamHost)

			if upstreamHost == "nxdomain" {
				// Return nxdomain asap
				msg.SetRcode(r, dns.RcodeNameError)
			} else {
				// Forward to the determined upstream dns server
				m := new(dns.Msg)
				m.SetQuestion(dns.Fqdn(domain), msg.Question[0].Qtype)
				m.RecursionDesired = true

				upstreamResponse, _, err := c.Exchange(m, net.JoinHostPort(upstreamHost, "53"))
				if upstreamResponse == nil {
					logger.Error("UpstreamError", err)
					return
				}

				if upstreamResponse.Rcode != dns.RcodeSuccess {
					msg.SetRcode(r, upstreamResponse.Rcode)
				} else {
					msg.Answer = upstreamResponse.Answer
					// Cache it
					memCache.Set(cacheKey, upstreamResponse.Answer, cache.DefaultExpiration)
				}
			}
		}
	}

	if writeErr := w.WriteMsg(&msg); writeErr != nil {
		logger.Error("HandlerWriteError", writeErr)
	}
}

func getDNSServerFromLookup(conf config.RouterConfig, domain string) string {
	dnsServer := conf.DefaultUpstream

	if len(conf.Upstreams) > 0 {
		for _, upstream := range conf.Upstreams {
			if found := upstream.CompiledRegex.MatchString(domain); found {
				if upstream.NXDomain {
					dnsServer = "nxdomain"
				} else {
					dnsServer = upstream.DNSServer
				}
				break
			}
		}
	}

	return dnsServer
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

func getDNSAnswerFromInternal(conf config.RouterConfig, m dns.Msg, domain string, serverIdx int) []dns.RR {
	if len(conf.InternalRecords) > 0 {
		rr := make([]dns.RR, 0)
		for _, internalRecord := range conf.InternalRecords {
			if found := internalRecord.CompiledRegex.MatchString(domain); found {
				switch m.Question[0].Qtype {
				case dns.TypeA:
					if internalRecord.A != "" {
						ip := net.ParseIP(internalRecord.A)
						rr = append(rr, &dns.A{Hdr: dns.RR_Header{Name: m.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET}, A: ip})
						logger.Debug("[%d] DNSLookup %s %s -> %s", serverIdx, domain, getRecordTypeString(m.Question[0].Qtype), internalRecord.A)
					}
				case dns.TypeAAAA:
					if internalRecord.AAAA != "" {
						ip := net.ParseIP(internalRecord.AAAA)
						rr = append(rr, &dns.AAAA{Hdr: dns.RR_Header{Name: m.Question[0].Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET}, AAAA: ip})
						logger.Debug("[%d] DNSLookup %s %s -> %s", serverIdx, domain, getRecordTypeString(m.Question[0].Qtype), internalRecord.AAAA)
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
				}
			}
			if len(rr) > 0 {
				break
			}
		}
		return rr
	}

	return nil
}
