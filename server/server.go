package server

import (
	"kimo/config"
	"sync"

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
	Pid              int32  `json:"pid,omitempty"`
	Host             string `json:"host"`
	Detail           string `json:"detail"`
}

// Server is a type for handling server side operations
type Server struct {
	Config           *config.ServerConfig
	PrometheusMetric *PrometheusMetric
	kimoProcesses    []KimoProcess
	Fetcher          *Fetcher

	mu        sync.RWMutex // proctects kimoProcesses
	AgentPort uint32
}

func (s *Server) SetKimoProcesses(kps []KimoProcess) {
	s.mu.Lock()
	s.kimoProcesses = kps
	s.mu.Unlock()
}

func (s *Server) GetKimoProcesses() []KimoProcess {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.kimoProcesses
}

// NewServer is used to create a new Server object
func NewServer(cfg *config.ServerConfig) *Server {
	log.Infoln("Creating a new server...")
	s := &Server{
		Config:           cfg,
		PrometheusMetric: NewPrometheusMetric(cfg.Metric.CmdlinePatterns),
		kimoProcesses:    make([]KimoProcess, 0),
		AgentPort:        cfg.Agent.Port,
	}
	s.Fetcher = NewFetcher(*s.Config)
	return s
}
