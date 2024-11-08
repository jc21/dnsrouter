package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"dnsrouter/internal/logger"
)

// DefaultUpstream is a system wide default
const DefaultUpstream = "1.1.1.1"

// ServerConfig ...
type ServerConfig struct {
	Log     LogConfig      `json:"log"`
	Servers []RouterConfig `json:"servers"`
	Cache   CacheConfig    `json:"cache"`
}

// CacheConfig is self explanatatory
type CacheConfig struct {
	Disabled bool  `json:"disabled"`
	Min      int64 `json:"min"`
	Max      int64 `json:"max"`
}

// RouterConfig is the settings that exist in the config file
type RouterConfig struct {
	Host            string                 `json:"host"`
	Port            int                    `json:"port"`
	Upstreams       []UpstreamConfig       `json:"upstreams"`
	InternalRecords []InternalRecordConfig `json:"internal"`
	DefaultUpstream string                 `json:"default_upstream"`
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

// InternalRecordConfig is a item for internal dns record configuration
type InternalRecordConfig struct {
	HostRegex     string         `json:"regex"`
	A             string         `json:"A"`
	AAAA          string         `json:"AAAA"`
	MX            string         `json:"MX"`
	TXT           string         `json:"TXT"`
	CompiledRegex *regexp.Regexp `json:"-"`
}

// NewServerConfig will create a new config instance and load it from the config file with defaults
func NewServerConfig() ServerConfig {
	s := ServerConfig{
		Log: LogConfig{
			Format: "nice",
			Level:  "info",
		},
		Cache: CacheConfig{
			Disabled: false,
			Min:      15,
			Max:      30,
		},
		Servers: []RouterConfig{},
	}
	s.Load()
	s.Check()
	initLogger()
	s.CompileRegexes()

	// Print out the config for debugging
	j, _ := json.MarshalIndent(s, "", " ")
	logger.Debug("%s", j)

	return s
}

func newDefaultRouter() RouterConfig {
	return RouterConfig{
		Host:            "127.0.0.1",
		Port:            53,
		DefaultUpstream: DefaultUpstream,
	}
}

// GetListenAddress Returns the listen address for the service
func (c *RouterConfig) GetListenAddress() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// Load will load the config from file
func (s *ServerConfig) Load() {
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
		contents, err = io.ReadAll(jsonFile)
		if err != nil {
			logger.Warn("Config file could not be read: %v", err.Error())
			ok = false
		}
	}

	if ok {
		err = json.Unmarshal(contents, &s)
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

	if s.Cache.Disabled {
		logger.Warn("Local cache is disabled")
	}
}

// CompileRegexes prepares the regexes given ahead of their usage
func (s *ServerConfig) CompileRegexes() {
	regexCount := 0
	iRegexCount := 0
	if len(s.Servers) > 0 {
		for sIdx, server := range s.Servers {
			for rIdx, upstream := range server.Upstreams {
				s.Servers[sIdx].Upstreams[rIdx].CompiledRegex = regexp.MustCompile(fmt.Sprintf("^%s\\.$", upstream.HostRegex))
				regexCount++
			}

			for iIdx, internalRecord := range server.InternalRecords {
				s.Servers[sIdx].InternalRecords[iIdx].CompiledRegex = regexp.MustCompile(fmt.Sprintf("^%s\\.$", internalRecord.HostRegex))
				iRegexCount++
			}
		}
	}

	logger.Info("Compiled %d upstream regexes and %d internal record regexes from %d servers", regexCount, iRegexCount, len(s.Servers))
}

// Check will ensure that the servers defined are not duplicated
func (s *ServerConfig) Check() {
	combinations := make([]string, 0)

	if len(s.Servers) == 0 {
		s.Servers = []RouterConfig{
			newDefaultRouter(),
		}
	} else {
		for idx, server := range s.Servers {
			if server.Host == "" {
				s.Servers[idx].Host = "127.0.0.1"
			}

			if server.Port == 0 {
				s.Servers[idx].Port = 53
			}

			thisCombination := fmt.Sprintf("%s:%d", strings.ToLower(s.Servers[idx].Host), s.Servers[idx].Port)
			if contains(combinations, thisCombination) {
				logger.Error("ServerConfigError", fmt.Errorf("Cannot start 2 servers with the same interface/port: %s", thisCombination))
				os.Exit(1)
			}

			combinations = append(combinations, thisCombination)
		}
	}
}

func contains(s []string, v string) bool {
	for _, a := range s {
		if a == v {
			return true
		}
	}
	return false
}
