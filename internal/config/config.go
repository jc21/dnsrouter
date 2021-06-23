package config

import (
	golog "log"

	"dnsrouter/internal/logger"

	"github.com/vrischmann/envconfig"
)

const (
	envPrefix = "DNSROUTER"
)

var (
	instance Config
	logLevel logger.Level
	// Commit is the git commit set by ldflags
	Commit string
	// Version is the version set by ldflags
	Version string
)

// Init will parse environment variables into the Env struct
func Init(version, commit *string) {
	Version = *version
	Commit = *commit

	if err := envconfig.InitWithPrefix(&instance, envPrefix); err != nil {
		logger.Error("EnvConfigError", err)
	}

	initLogger()
	logger.Info("Build Version: %s (%s)", Version, Commit)

	// TODO: this is debug
	instance.Upstreams = []UpstreamConfig{
		{
			HostRegex: ".*\\.jc21.example.com",
			DNSServer: "192.168.0.1",
		},
	}
}

// Get Returns the config object
func Get() Config {
	return instance
}

// Init initialises the Log object and return it
func initLogger() {
	// this removes timestamp prefixes from logs
	golog.SetFlags(0)

	switch instance.Log.Level {
	case "debug":
		logLevel = logger.DebugLevel
	case "warn":
		logLevel = logger.WarnLevel
	case "error":
		logLevel = logger.ErrorLevel
	default:
		logLevel = logger.InfoLevel
	}

	err := logger.Configure(&logger.Config{
		LogThreshold: logLevel,
		Formatter:    instance.Log.Format,
	})

	if err != nil {
		logger.Error("LoggerConfigurationError", err)
	}
}
