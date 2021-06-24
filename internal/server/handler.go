package server

import (
	"fmt"
	"net"
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
type DNSHandler struct{}

func initMemCache() {
	memCache = cache.New(30*time.Second, 1*time.Minute)
}

// ServeDNS will handle incoming dns requests and forward them onwards
func (hndlr *DNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	once.Do(initMemCache)
	c := new(dns.Client)

	msg := dns.Msg{}
	msg.SetReply(r)

	domain := msg.Question[0].Name

	// See if we have this cached
	cacheKey := fmt.Sprintf("%s-%d", dns.Fqdn(domain), msg.Question[0].Qtype)
	cacheItem, found := memCache.Get(cacheKey)

	if found {
		// Using our cached answer
		msg.Answer = cacheItem.([]dns.RR)
		logger.Debug("DNSLookup %s -> cached", domain)
	} else {
		upstreamHost := getDNSServerFromLookup(domain)
		logger.Debug("DNSLookup %s -> %s", domain, upstreamHost)

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

	if writeErr := w.WriteMsg(&msg); writeErr != nil {
		logger.Error("HandlerWriteError", writeErr)
	}
}

func getDNSServerFromLookup(domain string) string {
	conf := config.GetRouterConfig()
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
