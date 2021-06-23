package config

import (
	golog "log"

	"dnsrouter/internal/logger"

	c "github.com/JeremyLoy/config"
	"github.com/alexflint/go-arg"
)

var (
	appArguments ArgConfig
	routerConfig RouterConfig
	logLevel     logger.Level
	// Commit is the git commit set by ldflags
	Commit string
	// Version is the version set by ldflags
	Version string
)

const defaultConfigFile = "/etc/dnsrouter/config.json"

// GetConfig returns the ArgConfig
func Init(commit, version *string) {
	Version = *version
	Commit = *commit

	// nolint: errcheck
	c.FromEnv().To(&appArguments)
	arg.MustParse(&appArguments)

	routerConfig = NewRouterConfig()
}

func initLogger() {
	// this removes timestamp prefixes from logs
	golog.SetFlags(0)

	if appArguments.Verbose {
		logLevel = logger.DebugLevel
	} else {
		switch routerConfig.Log.Level {
		case "debug":
			logLevel = logger.DebugLevel
		case "warn":
			logLevel = logger.WarnLevel
		case "error":
			logLevel = logger.ErrorLevel
		default:
			logLevel = logger.InfoLevel
		}
	}

	err := logger.Configure(&logger.Config{
		LogThreshold: logLevel,
		Formatter:    routerConfig.Log.Format,
	})

	if err != nil {
		logger.Error("LoggerConfigurationError", err)
	}
}

func getConfigFilename() string {
	if appArguments.ConfigFile != "" {
		return appArguments.ConfigFile
	}

	return defaultConfigFile
}

// GetRouterConfig returns the configuration as read from a file
func GetRouterConfig() *RouterConfig {
	return &routerConfig
}
