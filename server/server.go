package server

import (
	"context"
	"kimo/config"
	"time"

	"github.com/cenkalti/log"
)

// KimoProcess is the final process that is combined with AgentProcess + TCPProxyConn + MysqlProcess
type KimoProcess struct {
	ID        int32    `json:"id"`
	MysqlUser string   `json:"mysql_user"`
	DB        string   `json:"db"`
	Command   string   `json:"command"`
	Time      uint32   `json:"time"`
	State     string   `json:"state"`
	Info      string   `json:"info"`
	CmdLine   []string `json:"cmdline"`
	Pid       int32    `json:"pid,omitempty"`
	Host      string   `json:"host"`
	Detail    string   `json:"detail"`
}

// Server is a type for handling server side operations
type Server struct {
	Config           *config.Server
	PrometheusMetric *PrometheusMetric
	KimoProcesses    []KimoProcess
	Fetcher          *Fetcher

	AgentPort uint32
}

// NewServer is used to create a new Server object
func NewServer(cfg *config.Config) *Server {
	log.Infoln("Creating a new server...")
	s := new(Server)
	s.Config = &cfg.Server
	s.PrometheusMetric = NewPrometheusMetric(s)
	s.KimoProcesses = make([]KimoProcess, 0)
	s.Fetcher = NewFetcher(*s.Config)

	s.AgentPort = cfg.Server.AgentPort
	return s
}

// GetProcesses gets all processes
func (s *Server) GetProcesses() {
	// todo: call with lock
	// todo: prevent race condition
	// todo: if a fetch is in progress and a new fetch is triggered, cancel the existing one.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ps, err := s.Fetcher.FetchAll(ctx)
	if err != nil {
		log.Error(err.Error())
		return
	}
	s.KimoProcesses = ps
	log.Debugf("%d processes are set\n", len(s.KimoProcesses))
}

func (s *Server) setMetrics() {
	// todo: prevent race condition
	s.PrometheusMetric.SetMetrics()
}

func (s *Server) pollAgents() {
	ticker := time.NewTicker(s.Config.PollDuration * time.Second)

	for {
		s.GetProcesses() // poll immediately at initialization
		select {
		// todo: add return case
		case <-ticker.C:
			s.GetProcesses()
		}
	}
}
