package server

import (
	"fmt"
	"net"

	"dnsrouter/internal/config"
	"dnsrouter/internal/logger"

	"github.com/miekg/dns"
)

type DNSHandler struct{}

func (this *DNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	c := new(dns.Client)

	msg := dns.Msg{}
	// msg.RecursionDesired = true
	msg.SetReply(r)

	domain := msg.Question[0].Name
	upstreamHost := getDNSServerFromLookup(domain)

	logger.Debug("DNSLookup %s -> %s", domain, upstreamHost)

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), msg.Question[0].Qtype)
	m.RecursionDesired = true

	upstreamResponse, _, err := c.Exchange(m, net.JoinHostPort(upstreamHost, "53"))
	if upstreamResponse == nil {
		logger.Error("UpstreamError", err)
		return
	}

	if upstreamResponse.Rcode != dns.RcodeSuccess {
		logger.Error("UpstreamDNSError", fmt.Errorf("ErrCode: %d", upstreamResponse.Rcode))
		return
	}

	msg.Answer = upstreamResponse.Answer
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
				dnsServer = upstream.DNSServer
				break
			}
		}
	}

	return dnsServer
}
