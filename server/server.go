package server

import (
	"kimo/config"

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
	Config           *config.ServerConfig
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
	s.PrometheusMetric = NewPrometheusMetric(cfg.Server.Metric.CmdlinePatterns)
	s.KimoProcesses = make([]KimoProcess, 0)
	s.Fetcher = NewFetcher(*s.Config)

	s.AgentPort = cfg.Server.Agent.Port
	return s
}
