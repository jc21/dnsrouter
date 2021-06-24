package config

// ArgConfig is the settings for passing arguments to the command
type ArgConfig struct {
	ConfigFile  string `arg:"-c" help:"Config File to use (default: /etc/dnsrouter/config.json)"`
	WriteConfig bool   `arg:"-w" help:"Write default configuration to the config file then exit"`
	Verbose     bool   `arg:"-v" help:"Print a lot more info (debug output)"`
}

// Description returns a simple description of the command
func (ArgConfig) Description() string {
	return "DNS upstream routing based on domain name lookups"
}
