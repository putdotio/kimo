package server

import (
	"context"
	"kimo/config"
	"net/http"
	"strconv"
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
	Pid              int    `json:"pid,omitempty"`
	Host             string `json:"host"`
	Detail           string `json:"detail"`
}

// Server is a type for handling server side operations
type Server struct {
	Config           *config.ServerConfig
	PrometheusMetric *PrometheusMetric
	Fetcher          *Fetcher
	AgentPort        uint32
	processes        []KimoProcess
	mu               sync.RWMutex // proctects processes
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

		// set agent process properties
		kp.CmdLine = rp.AgentProcess.CmdLine()
		kp.ConnectionStatus = rp.AgentProcess.ConnectionStatus()
		kp.Pid = rp.AgentProcess.Pid()
		kp.Host = rp.AgentProcess.Hostname()
		kp.Detail = rp.Detail()

		kps = append(kps, kp)
	}
	return kps
}

// NewServer creates an returns a new *Server
func NewServer(cfg *config.ServerConfig) *Server {
	log.Infoln("Creating a new server...")
	s := &Server{
		Config:           cfg,
		PrometheusMetric: NewPrometheusMetric(cfg.Metric.CmdlinePatterns),
		processes:        make([]KimoProcess, 0),
		AgentPort:        cfg.Agent.Port,
	}
	s.Fetcher = NewFetcher(*s.Config)
	return s
}

// Run starts the server and begins listening for HTTP requests.
func (s *Server) Run() error {
	log.Infof("Running server on %s \n", s.Config.ListenAddress)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.pollAgents(ctx)

	http.Handle("/", s.Static())
	http.Handle("/metrics", s.Metrics())
	http.HandleFunc("/procs", s.Procs)
	http.HandleFunc("/health", s.Health)
	err := http.ListenAndServe(s.Config.ListenAddress, nil)
	if err != nil {
		log.Errorln(err.Error())
		return err
	}
	return nil
}
