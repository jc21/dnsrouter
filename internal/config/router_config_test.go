package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const loopbackHost = "127.0.0.1"

func TestGetListenAddress(t *testing.T) {
	c := RouterConfig{Host: loopbackHost, Port: 5353}
	assert.Equal(t, "127.0.0.1:5353", c.GetListenAddress())
}

func TestCheck_NoServersGetsDefault(t *testing.T) {
	s := ServerConfig{}
	s.Check()

	require.Len(t, s.Servers, 1)
	assert.Equal(t, loopbackHost, s.Servers[0].Host)
	assert.Equal(t, 53, s.Servers[0].Port)
	assert.Equal(t, DefaultUpstream, s.Servers[0].DefaultUpstream)
}

func TestCheck_FillsInMissingHostAndPort(t *testing.T) {
	s := ServerConfig{Servers: []RouterConfig{{}}}
	s.Check()

	require.Len(t, s.Servers, 1)
	assert.Equal(t, loopbackHost, s.Servers[0].Host)
	assert.Equal(t, 53, s.Servers[0].Port)
}

func TestCompileRegexes_CompilesAndValidatesIPs(t *testing.T) {
	s := ServerConfig{
		Servers: []RouterConfig{
			{
				Upstreams: []UpstreamConfig{
					{HostRegex: ".*\\.example\\.com"},
				},
				InternalRecords: []InternalRecordConfig{
					{HostRegex: "mail\\.example\\.com", A: "192.168.0.10", AAAA: "not-a-valid-ipv6"},
				},
			},
		},
	}

	s.CompileRegexes()

	require.NotNil(t, s.Servers[0].Upstreams[0].CompiledRegex)
	assert.True(t, s.Servers[0].Upstreams[0].CompiledRegex.MatchString("www.example.com."))

	require.NotNil(t, s.Servers[0].InternalRecords[0].CompiledRegex)
	assert.Equal(t, "192.168.0.10", s.Servers[0].InternalRecords[0].A, "valid A record should be preserved")
	assert.Empty(t, s.Servers[0].InternalRecords[0].AAAA, "invalid AAAA record should be cleared rather than served broken")
}

func TestCompileHostRegex(t *testing.T) {
	re, err := compileHostRegex(".*\\.example\\.com")
	require.NoError(t, err)
	assert.True(t, re.MatchString("www.example.com."))
	assert.False(t, re.MatchString("www.example.com.evil.com."))

	_, err = compileHostRegex("(unbalanced")
	assert.Error(t, err, "a malformed regex must be reported as an error, not panic the process")
}

func TestLoad_MissingFileFallsBackToDefaults(t *testing.T) {
	appArguments.ConfigFile = filepath.Join(t.TempDir(), "does-not-exist.json")
	defer func() { appArguments.ConfigFile = "" }()

	s := ServerConfig{Cache: CacheConfig{Min: 15, Max: 30}}
	s.Load()

	assert.Empty(t, s.Servers, "no servers should be populated from a missing config file")
	assert.False(t, s.Cache.Disabled)
}

func TestLoad_ValidFilePopulatesConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"servers": [{"host": "0.0.0.0", "port": 5353}],
		"cache": {"disabled": true}
	}`), 0o600))

	appArguments.ConfigFile = path
	defer func() { appArguments.ConfigFile = "" }()

	s := ServerConfig{}
	s.Load()

	require.Len(t, s.Servers, 1)
	assert.Equal(t, "0.0.0.0", s.Servers[0].Host)
	assert.Equal(t, 5353, s.Servers[0].Port)
	assert.True(t, s.Cache.Disabled)
}

// The following tests exercise the fatal (os.Exit(1)) paths. Since those paths kill the
// process, each re-executes this same test binary as a subprocess and asserts on its exit
// code/status, rather than asserting on state after a call that would kill the test runner.

func TestLoad_MalformedJSONIsFatal(t *testing.T) {
	if os.Getenv("DNSROUTER_TEST_SUBPROCESS") == "malformed-json" {
		path := os.Getenv("DNSROUTER_TEST_CONFIG_FILE")
		appArguments.ConfigFile = path
		s := ServerConfig{}
		s.Load()
		return
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{ this is not valid json`), 0o600))

	err := runSubprocessTest(t, "TestLoad_MalformedJSONIsFatal", "malformed-json", path)
	var exitErr *exec.ExitError
	require.ErrorAsf(t, err, &exitErr, "expected the process to exit non-zero on malformed config JSON, got: %v", err)
	assert.False(t, exitErr.Success())
}

func TestCompileRegexes_InvalidRegexIsFatal(t *testing.T) {
	if os.Getenv("DNSROUTER_TEST_SUBPROCESS") == "invalid-regex" {
		s := ServerConfig{Servers: []RouterConfig{{Upstreams: []UpstreamConfig{{HostRegex: "(unbalanced"}}}}}
		s.CompileRegexes()
		return
	}

	err := runSubprocessTest(t, "TestCompileRegexes_InvalidRegexIsFatal", "invalid-regex", "")
	var exitErr *exec.ExitError
	require.ErrorAsf(t, err, &exitErr, "expected the process to exit non-zero on an invalid config regex, got: %v", err)
	assert.False(t, exitErr.Success())
}

func TestCheck_DuplicateServerIsFatal(t *testing.T) {
	if os.Getenv("DNSROUTER_TEST_SUBPROCESS") == "duplicate-server" {
		s := ServerConfig{Servers: []RouterConfig{
			{Host: loopbackHost, Port: 53},
			{Host: loopbackHost, Port: 53},
		}}
		s.Check()
		return
	}

	err := runSubprocessTest(t, "TestCheck_DuplicateServerIsFatal", "duplicate-server", "")
	var exitErr *exec.ExitError
	require.ErrorAsf(t, err, &exitErr, "expected the process to exit non-zero on duplicate host:port servers, got: %v", err)
	assert.False(t, exitErr.Success())
}

// runSubprocessTest re-executes the current test binary, running only the named test, with
// DNSROUTER_TEST_SUBPROCESS set so that test's subprocess branch (above) runs instead of its
// normal assertions.
func runSubprocessTest(t *testing.T, testName, mode, configFile string) error {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^"+testName+"$")
	cmd.Env = append(os.Environ(), "DNSROUTER_TEST_SUBPROCESS="+mode, "DNSROUTER_TEST_CONFIG_FILE="+configFile)
	out, err := cmd.CombinedOutput()
	t.Logf("subprocess output:\n%s", out)
	return err
}
