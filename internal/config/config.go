// Package config ...
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"dnsrouter/internal/logger"

	c "github.com/JeremyLoy/config"
	"github.com/alexflint/go-arg"
)

var (
	appArguments ArgConfig
	serverConfig ServerConfig
	logLevel     logger.Level
	// Commit is the git commit set by ldflags
	Commit string
	// Version is the version set by ldflags
	Version string
)

const defaultConfigFile = "/etc/dnsrouter/config.json"

// Init will parse arg vars and setup the config for the app
func Init(commit, version *string) {
	Version = *version
	Commit = *commit

	// nolint: errcheck, gosec
	c.FromEnv().To(&appArguments)
	arg.MustParse(&appArguments)

	serverConfig = NewServerConfig()

	if appArguments.WriteConfig {
		writeConfig()
	}
}

func initLogger() {
	if appArguments.Verbose {
		logLevel = logger.DebugLevel
	} else {
		switch serverConfig.Log.Level {
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
		Formatter:    serverConfig.Log.Format,
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

// GetServerConfig returns the configuration as read from a file
func GetServerConfig() *ServerConfig {
	return &serverConfig
}

// writeConfig will write/amend the config file and exit
func writeConfig() {
	filename := getConfigFilename()
	content, _ := json.MarshalIndent(serverConfig, "", " ")

	// Make sure the parent folder exists
	folder := path.Dir(filename)

	// nolint: gosec
	dirErr := os.MkdirAll(folder, os.ModePerm)
	if dirErr != nil {
		logger.Error("ConfigWriteError", fmt.Errorf("Could not create config folder: %s: %s", path.Dir(filename), dirErr.Error()))
		os.Exit(1)
	}

	err := os.WriteFile(filename, content, 0600)
	if err != nil {
		logger.Error("ConfigWriteError", fmt.Errorf("Could not write config file: %s: %s", filename, err.Error()))
		os.Exit(1)
	}

	logger.Info("Successfully wrote: %s", filename)
	os.Exit(0)
}
