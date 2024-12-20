package server

import (
	"context"
	"fmt"
	"kimo/config"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/cenkalti/log"
)

// KimoProcess is the final process that is combined with AgentProcess + TCPProxyConn + MysqlProcess
type KimoProcess struct {
	ID               int32  `json:"id"`
	MysqlUser        string `json:"mysql_user"`
	DB               string `json:"db"`
	Command          string `json:"command"`
	Time             uint32 `json:"time"`
	State            string `json:"state"`
	Info             string `json:"info"`
	CmdLine          string `json:"cmdline"`
	ConnectionStatus string `json:"status"`
	Pid              int    `json:"pid,omitempty"`
	Host             string `json:"host"`
	Detail           string `json:"detail"`
}

// Server is a type for handling server side operations
type Server struct {
	Config             *config.ServerConfig
	PrometheusMetric   *PrometheusMetric
	Fetcher            *Fetcher
	AgentListenPort    uint32
	processes          []KimoProcess
	mu                 sync.RWMutex // proctects processes
	lastSuccessfulPoll time.Time
	lastPollError      error
	healthMutex        sync.RWMutex
	httpSrv            http.Server
}

// SetProcesses sets kimo processes with lock
func (s *Server) SetProcesses(kps []KimoProcess) {
	s.mu.Lock()
	s.processes = kps
	s.mu.Unlock()
}

// GetProcesses gets kimo processes with lock
func (s *Server) GetProcesses() []KimoProcess {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.processes
}

// ConvertProcesses convert raw processes to kimo processes
func (s *Server) ConvertProcesses(rps []*RawProcess) []KimoProcess {
	kps := make([]KimoProcess, 0)
	for _, rp := range rps {
		ut, err := strconv.ParseUint(rp.MysqlRow.Time, 10, 32)
		if err != nil {
			log.Errorf("time %s could not be converted to int", rp.MysqlRow.Time)
		}
		var kp KimoProcess

		// set mysql properties
		kp.ID = rp.MysqlRow.ID
		kp.MysqlUser = rp.MysqlRow.User
		kp.DB = rp.MysqlRow.DB.String
		kp.Command = rp.MysqlRow.Command
		kp.Time = uint32(ut)
		kp.State = rp.MysqlRow.State.String
		kp.Info = rp.MysqlRow.Info.String

		// set process properties
		if rp.Process != nil {
			kp.CmdLine = rp.Process.Cmdline
			kp.ConnectionStatus = rp.Process.ConnectionStatus
			kp.Pid = int(rp.Process.Pid)
			kp.Host = rp.Process.Host()
		}

		// set misc.
		kp.Detail = rp.Detail()

		kps = append(kps, kp)
	}
	return kps
}

// NewServer creates an returns a new *Server
func NewServer(cfg *config.ServerConfig) *Server {
	s := &Server{
		Config:           cfg,
		PrometheusMetric: NewPrometheusMetric(cfg.Metric.CmdlinePatterns),
		processes:        make([]KimoProcess, 0),
		AgentListenPort:  cfg.Agent.Port,
	}
	s.Fetcher = NewFetcher(*s.Config)

	// create http server
	mux := http.NewServeMux()
	mux.Handle("/", s.Static())
	mux.Handle("/metrics", s.Metrics())
	mux.HandleFunc("/procs", s.Procs)
	mux.HandleFunc("/health", s.Health)
	s.httpSrv = http.Server{
		Addr:    s.Config.ListenAddress,
		Handler: mux,
	}

	return s
}

// Run starts the server and begins listening for HTTP requests.
func (s *Server) Run() error {
	log.Infof("Running server on %s \n", s.Config.ListenAddress)

	errChan := make(chan error, 1)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start server in a goroutine
	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("http server error: %w", err)
		}
	}()

	// Poll kimo agents in a goroutine
	go func() {
		if err := s.pollAgents(ctx); err != nil {
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

	if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Infof("HTTP server shutdown failed: %v", err)
		return err
	}
	log.Infoln("Server gracefully stopped")
	return nil
}
