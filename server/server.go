package server

import (
	"context"
	"kimo/config"
	"time"

	"github.com/cenkalti/log"
)

// Process is the final processes that is combined with AgentProcess + TCPProxyRecord + MysqlProcess
type Process struct {
	ID        int32    `json:"id"`
	MysqlUser string   `json:"mysql_user"`
	DB        string   `json:"db"`
	Command   string   `json:"command"`
	Time      uint32   `json:"time"`
	State     string   `json:"state"`
	Info      string   `json:"info"`
	CmdLine   []string `json:"cmdline"`
	Pid       int32    `json:"pid"`
	Host      string   `json:"host"`
}

// Server is a type for handling server side
type Server struct {
	Config           *config.Server
	PrometheusMetric *PrometheusMetric
	Processes        []Process // todo: bad naming.
	Client           *Client
}

// NewServer is used to create a new Server type
func NewServer(cfg *config.Config) *Server {
	log.Infoln("Creating a new server...")
	s := new(Server)
	s.Config = &cfg.Server
	s.PrometheusMetric = NewPrometheusMetric(s)
	s.Processes = make([]Process, 0)
	s.Client = NewClient(*s.Config)
	return s
}

// FetchAll fetches all processes through Client object
func (s *Server) FetchAll() {
	// todo: call with lock
	// todo: prevent race condition
	// todo: if a fetch is in progress and a new fetch is triggered, cancel the existing one.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ps, err := s.Client.FetchAll(ctx)
	if err != nil {
		log.Error(err.Error())
		return
	}
	s.Processes = ps
	log.Debugf("%d processes are set\n", len(s.Processes))
}

func (s *Server) setMetrics() {
	// todo: prevent race condition
	s.PrometheusMetric.SetMetrics()
}

func (s *Server) pollAgents() {
	ticker := time.NewTicker(s.Config.PollDuration * time.Second)

	for {
		s.FetchAll() // poll immediately at initialization
		select {
		// todo: add return case
		case <-ticker.C:
			s.FetchAll()
		}
	}

}
