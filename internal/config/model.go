package config

import (
	"net"
	"strconv"

	"dnsrouter/internal/logger"
)

// Config The Config struct
type Config struct {
	Host            string           `json:"host" envconfig:"optional,default=0.0.0.0"`
	Port            int              `json:"port" envconfig:"optional,default=53"`
	Log             LogConfig        `json:"log" envconfig:"optional"`
	Upstreams       []UpstreamConfig `json:"upstreams" envconfig:"optional"`
	DefaultUpstream string           `json:"default_upstream" envconfig:"optional,default=1.1.1.1"`
}

// LogConfig ...
type LogConfig struct {
	Format string `json:"format" envconfig:"optional,default=info"`
	Level  string `json:"level" envconfig:"optional,default=json"`
}

type UpstreamConfig struct {
	HostRegex string `json:"host_regex"`
	DNSServer string `json:"dns_server"`
}

// GetListenAddress Returns the listen address for the service
func (c Config) GetListenAddress() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// GetLoggerLevel translates a level string into a level constant used by jumgo logger
func (c Config) GetLoggerLevel() logger.Level {
	switch c.Log.Level {
	case "debug":
		return logger.DebugLevel
	case "warn":
		return logger.WarnLevel
	case "error":
		return logger.ErrorLevel
	default:
		return logger.InfoLevel
	}
}
