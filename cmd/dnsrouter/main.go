// Package main ...
package main

import (
	golog "log"
	"os"
	"sync"

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
	conf := config.GetServerConfig()

	wg := new(sync.WaitGroup)
	wg.Add(len(conf.Servers))

	// Create a new DNS server for all servers
	for idx, routerConfig := range conf.Servers {
		go listenAndServe(idx, routerConfig, wg)
	}

	wg.Wait()
}

func listenAndServe(idx int, routerConfig config.RouterConfig, wg *sync.WaitGroup) {
	srv := &dns.Server{Addr: routerConfig.GetListenAddress(), Net: "udp"}
	srv.Handler = &server.DNSHandler{
		ServerIndex: idx,
		RouterConf:  routerConfig,
	}
	logger.Info("Server %d Listening on %s", idx, routerConfig.GetListenAddress())

	if err := srv.ListenAndServe(); err != nil {
		logger.Error("ListenError", err)
		os.Exit(1)
	}
	// nolint: errcheck
	defer srv.Shutdown()
	wg.Done()
}
