package server

import (
	"fmt"
	"io"
	stdlog "log"
	"net"
	"os"
	"regexp"
	"testing"
	"time"

	"dnsrouter/internal/config"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeResponseWriter is a minimal dns.ResponseWriter that just captures whatever
// ServeDNS writes back, so ServeDNS can be exercised without any real network I/O.
type fakeResponseWriter struct {
	written  *dns.Msg
	writeErr error
}

func (*fakeResponseWriter) LocalAddr() net.Addr       { return nil }
func (*fakeResponseWriter) RemoteAddr() net.Addr      { return nil }
func (*fakeResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (*fakeResponseWriter) Close() error              { return nil }
func (*fakeResponseWriter) TsigStatus() error         { return nil }
func (*fakeResponseWriter) TsigTimersOnly(bool)       {}
func (*fakeResponseWriter) Hijack()                   {}

func (f *fakeResponseWriter) WriteMsg(m *dns.Msg) error {
	f.written = m
	return f.writeErr
}

// mustCompileHostRegex mirrors how internal/config.CompileRegexes anchors a configured
// host regex, without importing that package (which would create an import cycle).
func mustCompileHostRegex(t *testing.T, pattern string) *regexp.Regexp {
	t.Helper()
	re, err := regexp.Compile(fmt.Sprintf("^%s\\.$", pattern))
	require.NoError(t, err)
	return re
}

func question(name string, qtype uint16) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), qtype)
	return m
}

func TestGetDNSServerFromLookup(t *testing.T) {
	conf := config.RouterConfig{
		DefaultUpstream: "1.1.1.1",
		Upstreams: []config.UpstreamConfig{
			{HostRegex: "local", NXDomain: true, CompiledRegex: mustCompileHostRegex(t, "local")},
			{HostRegex: ".*\\.example\\.com", DNSServer: "8.8.8.8", CompiledRegex: mustCompileHostRegex(t, ".*\\.example\\.com")},
		},
	}

	t.Run("no match falls back to default upstream", func(t *testing.T) {
		host, nxdomain := getDNSServerFromLookup(conf, "somewhere.else.")
		assert.Equal(t, "1.1.1.1", host)
		assert.False(t, nxdomain)
	})

	t.Run("matched rule returns its upstream", func(t *testing.T) {
		host, nxdomain := getDNSServerFromLookup(conf, "www.example.com.")
		assert.Equal(t, "8.8.8.8", host)
		assert.False(t, nxdomain)
	})

	t.Run("matched nxdomain rule", func(t *testing.T) {
		host, nxdomain := getDNSServerFromLookup(conf, "local.")
		assert.True(t, nxdomain)
		assert.Empty(t, host)
	})
}

func TestGetDNSAnswerFromInternal(t *testing.T) {
	conf := config.RouterConfig{
		InternalRecords: []config.InternalRecordConfig{
			{
				HostRegex:     "mail\\.example\\.com",
				A:             "192.168.0.10",
				TXT:           "hello",
				CompiledRegex: mustCompileHostRegex(t, "mail\\.example\\.com"),
			},
		},
	}

	t.Run("unmatched domain falls through", func(t *testing.T) {
		rr, matched := getDNSAnswerFromInternal(conf, *question("other.example.com.", dns.TypeA), "other.example.com.", 0)
		assert.False(t, matched)
		assert.Empty(t, rr)
	})

	t.Run("matched domain with configured qtype answers", func(t *testing.T) {
		rr, matched := getDNSAnswerFromInternal(conf, *question("mail.example.com.", dns.TypeA), "mail.example.com.", 0)
		require.True(t, matched)
		require.Len(t, rr, 1)
		a, ok := rr[0].(*dns.A)
		require.True(t, ok)
		assert.Equal(t, "192.168.0.10", a.A.String())
	})

	t.Run("matched domain without an answer for this qtype is NODATA, not a miss", func(t *testing.T) {
		// mail.example.com has no AAAA configured. Since it matched the internal
		// record, this must be reported as "matched" (so ServeDNS answers NODATA)
		// rather than falling through to the public upstream.
		rr, matched := getDNSAnswerFromInternal(conf, *question("mail.example.com.", dns.TypeAAAA), "mail.example.com.", 0)
		assert.True(t, matched)
		assert.Empty(t, rr)
	})

	t.Run("invalid IP literal in config is defensively skipped", func(t *testing.T) {
		bad := config.RouterConfig{
			InternalRecords: []config.InternalRecordConfig{
				{HostRegex: "bad\\.example\\.com", A: "not-an-ip", CompiledRegex: mustCompileHostRegex(t, "bad\\.example\\.com")},
			},
		}
		rr, matched := getDNSAnswerFromInternal(bad, *question("bad.example.com.", dns.TypeA), "bad.example.com.", 0)
		assert.True(t, matched)
		assert.Empty(t, rr)
	})
}

func TestServeDNS_CaseInsensitiveMatching(t *testing.T) {
	conf := config.RouterConfig{
		InternalRecords: []config.InternalRecordConfig{
			{HostRegex: "mail\\.example\\.com", A: "192.168.0.10", CompiledRegex: mustCompileHostRegex(t, "mail\\.example\\.com")},
		},
	}
	h := &DNSHandler{RouterConf: conf}

	w := &fakeResponseWriter{}
	// Mixed-case query (as a resolver doing 0x20-randomization might send) must still
	// match the lower-case configured rule.
	h.ServeDNS(w, question("MaIl.ExAmPlE.CoM.", dns.TypeA))

	require.NotNil(t, w.written)
	require.Len(t, w.written.Answer, 1)
	a, ok := w.written.Answer[0].(*dns.A)
	require.True(t, ok)
	assert.Equal(t, "192.168.0.10", a.A.String())
}

func TestServeDNS_SurvivesUnexpectedCrash(t *testing.T) {
	// The recovered value's own text is the verbatim Go runtime string "runtime error:
	// invalid memory address or nil pointer dereference". ServeDNS logs it (as it should -
	// operators need to see it), but go test captures whatever the standard "log" package
	// writes as this test's own output, and tparse (used by scripts/unit.sh) flags any
	// captured line containing "runtime error:" as an unrecovered panic. Redirect it during
	// this test only, exactly as logger_test.go does, so a deliberately-recovered crash
	// doesn't get misreported as one that wasn't.
	stdlog.SetOutput(io.Discard)
	defer stdlog.SetOutput(os.Stderr)

	h := &DNSHandler{}
	w := &fakeResponseWriter{}

	assert.NotPanics(t, func() {
		// A nil request causes a nil-pointer dereference inside ServeDNS; a single bad
		// packet must not be able to take the whole process down.
		h.ServeDNS(w, nil)
	})
}

func TestCache_GetSetRoundTripsAndIsolatesCopies(t *testing.T) {
	c := NewCache(config.CacheConfig{Min: 1, Max: 2})
	require.NotNil(t, c)

	rr := &dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300}, A: net.ParseIP("1.2.3.4")}
	c.set("key", []dns.RR{rr})

	got1, found := c.get("key")
	require.True(t, found)
	require.Len(t, got1, 1)

	// Mutate the RR handed back from one "request" and make sure a second, concurrent
	// "request" doesn't see the mutation - this is exactly the shared-mutable-state
	// data race that used to exist when the raw cached slice was handed out directly.
	got1[0].Header().Rdlength = 9999

	got2, found := c.get("key")
	require.True(t, found)
	require.Len(t, got2, 1)
	assert.NotEqual(t, uint16(9999), got2[0].Header().Rdlength)
}

func TestCache_Disabled(t *testing.T) {
	assert.Nil(t, NewCache(config.CacheConfig{Disabled: true}))
}

func TestCache_TTLWithinConfiguredBounds(t *testing.T) {
	c := NewCache(config.CacheConfig{Min: 1, Max: 3})
	require.NotNil(t, c)

	c.set("key", []dns.RR{})

	items := c.store.Items()
	item, ok := items["key"]
	require.True(t, ok)

	ttl := time.Duration(item.Expiration - time.Now().UnixNano())
	assert.GreaterOrEqual(t, ttl, c.minTTL-time.Second) // small slack for test execution time
	assert.LessOrEqual(t, ttl, c.maxTTL)
}

func TestGetRecordTypeString(t *testing.T) {
	assert.Equal(t, "A", getRecordTypeString(dns.TypeA))
	assert.Equal(t, "AAAA", getRecordTypeString(dns.TypeAAAA))
	assert.Equal(t, "TXT", getRecordTypeString(dns.TypeTXT))
	assert.Equal(t, "MX", getRecordTypeString(dns.TypeMX))
	assert.Equal(t, fmt.Sprintf("%v", dns.TypeNS), getRecordTypeString(dns.TypeNS))
}
