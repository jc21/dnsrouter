// Package main ...
package main

import (
	"context"
	golog "log"
	"os"
	"os/signal"
	"sync"
	"syscall"

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

	// The answer cache is process-wide (there's a single "cache" section in the config
	// file, not one per server), so it's built once here and shared across every
	// listener rather than being lazily/implicitly built from whichever server's first
	// packet happens to win a race.
	sharedCache := server.NewCache(conf.Cache)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	wg := new(sync.WaitGroup)
	// Each configured server gets both a UDP and a TCP listener sharing one handler: a
	// well-behaved client that receives a truncated (TC=1) UDP answer is expected to
	// retry the same query over TCP against the same server, but until now dnsrouter
	// only ever bound UDP, so that retry would just get connection-refused.
	wg.Add(len(conf.Servers) * 2)

	servers := make([]*dns.Server, 0, len(conf.Servers)*2)
	// Buffered so every listenAndServe goroutine can report a fatal error (e.g. address
	// already in use) without blocking, even if nothing is receiving from it yet.
	fatal := make(chan error, len(conf.Servers)*2)

	for idx, routerConfig := range conf.Servers {
		handler := &server.DNSHandler{
			ServerIndex: idx,
			RouterConf:  routerConfig,
			Cache:       sharedCache,
		}

		for _, proto := range []string{"udp", "tcp"} {
			srv := &dns.Server{Addr: routerConfig.GetListenAddress(), Net: proto, Handler: handler}
			servers = append(servers, srv)
			go listenAndServe(idx, srv, wg, fatal)
		}
	}

	// Shut every listener down gracefully - on SIGINT/SIGTERM (e.g. `systemctl stop`),
	// or if any one of them reports a fatal startup error - instead of either being
	// killed with no shutdown log line, or one failed listener taking the whole process
	// down via a bare os.Exit while its siblings are still serving traffic.
	go func() {
		var err error
		select {
		case <-ctx.Done():
			logger.Info("Shutting down")
		case err = <-fatal:
			logger.Error("ListenError", err)
		}

		for _, srv := range servers {
			if shutdownErr := srv.Shutdown(); shutdownErr != nil {
				logger.Error("ShutdownError", shutdownErr)
			}
		}

		if err != nil {
			os.Exit(1)
		}
	}()

	wg.Wait()
}

func listenAndServe(idx int, srv *dns.Server, wg *sync.WaitGroup, fatal chan<- error) {
	defer wg.Done()

	logger.Info("Server %d Listening on %s/%s", idx, srv.Addr, srv.Net)

	if err := srv.ListenAndServe(); err != nil {
		fatal <- err
	}
}
