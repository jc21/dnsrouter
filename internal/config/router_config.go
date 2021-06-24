package config

import (
	"dnsrouter/internal/logger"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"regexp"
	"strconv"
)

// RouterConfig is the settings that exist in the config file
type RouterConfig struct {
	Host            string           `json:"host"`
	Port            int              `json:"port"`
	Log             LogConfig        `json:"log"`
	Upstreams       []UpstreamConfig `json:"upstreams"`
	DefaultUpstream string           `json:"default_upstream"`
}

// LogConfig is self explanatatory
type LogConfig struct {
	Format string `json:"format"`
	Level  string `json:"level"`
}

// UpstreamConfig is a item for upstream configuration
type UpstreamConfig struct {
	HostRegex     string         `json:"regex"`
	DNSServer     string         `json:"upstream"`
	NXDomain      bool           `json:"nxdomain"`
	CompiledRegex *regexp.Regexp `json:"-"`
}

// NewRouterConfig will create a new config instance and load it from the config file with defaults
func NewRouterConfig() RouterConfig {
	r := RouterConfig{
		Host: "0.0.0.0",
		Port: 53,
		Log: LogConfig{
			Format: "nice",
			Level:  "info",
		},
		DefaultUpstream: "1.1.1.1",
	}
	r.Load()
	initLogger()
	r.CompileRegexes()
	return r
}

// GetListenAddress Returns the listen address for the service
func (c *RouterConfig) GetListenAddress() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// Load will load the config from file
func (c *RouterConfig) Load() {
	filename := getConfigFilename()
	ok := true

	// Make sure file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		logger.Warn("Config file not found, expected to find it at %s", filename)
		ok = false
	}

	var jsonFile *os.File
	var err error
	var contents []byte

	if ok {
		jsonFile, err = os.Open(path.Clean(filename))
		if err != nil {
			logger.Warn("Config file could not be opened: %v", err.Error())
			ok = false
		}
	}

	if ok {
		contents, err = ioutil.ReadAll(jsonFile)
		if err != nil {
			logger.Warn("Config file could not be read: %v", err.Error())
			ok = false
		}
	}

	if ok {
		err = json.Unmarshal(contents, &c)
		if err != nil {
			logger.Warn("Config file looks damaged")
			ok = false
		}
	}

	// nolint: errcheck, gosec
	if jsonFile != nil {
		jsonFile.Close()
	}

	if !ok {
		logger.Warn("Falling back to default configuration")
	}
}

// CompileRegexes prepares the regexes given ahead of their usage
func (c *RouterConfig) CompileRegexes() {
	if len(c.Upstreams) > 0 {
		for idx, upstream := range c.Upstreams {
			c.Upstreams[idx].CompiledRegex = regexp.MustCompile(fmt.Sprintf("^%s\\.$", upstream.HostRegex))
		}
	}

	logger.Info("Compiled %d upstream regexes", len(c.Upstreams))
}
