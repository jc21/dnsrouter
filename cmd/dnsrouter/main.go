package main

import (
	golog "log"
	"os"

	"dnsrouter/internal/config"
	"dnsrouter/internal/logger"
	"dnsrouter/internal/server"

	"github.com/miekg/dns"
)

var (
	commit  string
	version string
)

func main() {
	// this removes timestamp prefixes from logs
	golog.SetFlags(0)

	config.Init(&version, &commit)
	conf := config.GetRouterConfig()

	srv := &dns.Server{Addr: conf.GetListenAddress(), Net: "udp"}
	srv.Handler = &server.DNSHandler{}
	logger.Info("Listening on %s", conf.GetListenAddress())

	if err := srv.ListenAndServe(); err != nil {
		logger.Error("ListenError", err)
		os.Exit(1)
	}

	// nolint: errcheck
	defer srv.Shutdown()
}
