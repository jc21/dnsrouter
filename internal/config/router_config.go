package config

import (
	"dnsrouter/internal/logger"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
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

// LogConfig ...
type LogConfig struct {
	Format string `json:"format"`
	Level  string `json:"level"`
}

type UpstreamConfig struct {
	HostRegex     string         `json:"regex"`
	DNSServer     string         `json:"upstream"`
	CompiledRegex *regexp.Regexp `json:"-"`
}

// NewRouterConfig ...
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

// GetRouterConfig returns the configuration as read from a file
func (c *RouterConfig) Load() {
	filename := getConfigFilename()

	// Make sure file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		logger.Error("ConfigFileError", fmt.Errorf("Configuration not found, expected to find it at %s", filename))
		os.Exit(1)
	}

	jsonFile, err := os.Open(filename)
	if err != nil {
		logger.Error("ConfigFileError", fmt.Errorf("Configuration could not be opened: %v", err.Error()))
		os.Exit(1)
	}

	defer jsonFile.Close()

	contents, readErr := ioutil.ReadAll(jsonFile)
	if readErr != nil {
		logger.Error("ConfigFileError", fmt.Errorf("Configuration file could not be read: %v", readErr.Error()))
		os.Exit(1)
	}

	unmarshalErr := json.Unmarshal(contents, &c)
	if unmarshalErr != nil {
		logger.Error("ConfigFileError", fmt.Errorf("Configuration file looks damaged"))
		os.Exit(1)
	}
}

// CompileRegexes prepares the regexes given
func (c *RouterConfig) CompileRegexes() {
	if len(c.Upstreams) > 0 {
		for idx, upstream := range c.Upstreams {
			c.Upstreams[idx].CompiledRegex = regexp.MustCompile(fmt.Sprintf("^%s\\.$", upstream.HostRegex))
		}
	}

	logger.Info("Compiled %d upstream regexes", len(c.Upstreams))
}
