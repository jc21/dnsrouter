package main

import (
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
	config.Init(&version, &commit)
	conf := config.GetRouterConfig()

	srv := &dns.Server{Addr: conf.GetListenAddress(), Net: "udp"}
	srv.Handler = &server.DNSHandler{}
	logger.Info("Listening on %s", conf.GetListenAddress())

	if err := srv.ListenAndServe(); err != nil {
		logger.Error("ListenError", err)
		os.Exit(1)
	}

	defer srv.Shutdown()
}
