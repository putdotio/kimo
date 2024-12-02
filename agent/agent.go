package agent

import (
	"context"
	"fmt"
	"kimo/config"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cenkalti/log"
	gopsutilNet "github.com/shirou/gopsutil/v4/net"
)

// Agent is type for handling agent operations
type Agent struct {
	Config   *config.AgentConfig
	conns    []Conn
	Hostname string
	mu       sync.RWMutex // protects conns
	httpSrv  http.Server
}

type Conn struct {
	Port   uint32
	Pid    int32
	Status string
}

// NewAgent creates an returns a new Agent
func NewAgent(cfg *config.AgentConfig) *Agent {
	a := &Agent{
		Config:   cfg,
		Hostname: getHostname(),
	}

	// create http server
	mux := http.NewServeMux()
	mux.HandleFunc("/proc", a.Process)
	a.httpSrv = http.Server{
		Addr:    a.Config.ListenAddress,
		Handler: mux,
	}

	return a
}

// SetConns sets connections with lock.
func (a *Agent) SetConns(conns []Conn) {
	a.mu.Lock()
	a.conns = conns
	a.mu.Unlock()
}

// GetConns gets connections with lock.
func (a *Agent) GetConns() []Conn {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.conns
}

func (a *Agent) ConvertConns(gopsConns []gopsutilNet.ConnectionStat) []Conn {
	conns := make([]Conn, 0)
	for _, cs := range gopsConns {
		conn := Conn{
			Port:   cs.Laddr.Port,
			Status: cs.Status,
			Pid:    cs.Pid,
		}
		conns = append(conns, conn)
	}
	return conns
}

// getHostname returns hostname.
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		log.Errorf("Hostname could not found")
		hostname = "UNKNOWN"
	}
	return hostname
}

// Run starts the http server and begins listening for HTTP requests.
func (a *Agent) Run() error {
	log.Infof("Running server on %s \n", a.Config.ListenAddress)

	errChan := make(chan error, 1)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start server in a goroutine
	go func() {
		if err := a.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("http server error: %w", err)
		}
	}()

	// Poll connections in a goroutine
	go func() {
		if err := a.pollConns(ctx); err != nil {
			errChan <- fmt.Errorf("agent polling error: %w", err)
		}
	}()

	// Wait for interrupt signal or error
	select {
	case <-ctx.Done():
		log.Infof("Received signal, starting shutdown...")
	case err := <-errChan:
		log.Infof("Received error: %v, starting shutdown...", err)
		return err
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Infof("HTTP server shutdown failed: %v", err)
		return err
	}

	log.Infoln("Server gracefully stopped")
	return nil
}
