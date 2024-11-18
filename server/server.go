package server

import (
	"context"
	"kimo/config"
	"strconv"
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

	rps, err := s.Fetcher.FetchAll(ctx)
	if err != nil {
		log.Error(err.Error())
		return
	}
	s.KimoProcesses = s.createKimoProcesses(rps)

	log.Debugf("%d processes are set\n", len(s.KimoProcesses))
}

func (s *Server) setMetrics() {
	// todo: prevent race condition
	s.PrometheusMetric.SetMetrics()
}

func (s *Server) pollAgents() {
	ticker := time.NewTicker(s.Config.PollInterval * time.Second)

	for {
		s.GetProcesses() // poll immediately at initialization
		select {
		// todo: add return case
		case <-ticker.C:
			s.GetProcesses()
		}
	}
}

func (s *Server) createKimoProcesses(rps []*RawProcess) []KimoProcess {
	kps := make([]KimoProcess, 0)
	for _, rp := range rps {
		ut, err := strconv.ParseUint(rp.MysqlRow.Time, 10, 32)
		if err != nil {
			log.Errorf("time %s could not be converted to int", rp.MysqlRow.Time)
		}
		kp := KimoProcess{
			ID:        rp.MysqlRow.ID,
			MysqlUser: rp.MysqlRow.User,
			DB:        rp.MysqlRow.DB.String,
			Command:   rp.MysqlRow.Command,
			Time:      uint32(ut),
			State:     rp.MysqlRow.State.String,
			Info:      rp.MysqlRow.Info.String,
		}
		if rp.AgentProcess != nil {
			kp.CmdLine = rp.AgentProcess.CmdLine
			kp.Pid = rp.AgentProcess.Pid
			kp.Host = rp.AgentProcess.Hostname
		} else {
			if rp.Details != nil {
				kp.Detail = rp.Detail()
			}
		}
		kps = append(kps, kp)
	}
	return kps
}
