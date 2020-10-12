package server

import (
	"context"
	"encoding/json"
	"fmt"
	"kimo/config"
	"net/http"
	"time"

	"github.com/cenkalti/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rakyll/statik/fs"

	_ "kimo/statik" // Auto-generated module by statik.
)

// Response is type for returning a response from kimo server
type Response struct {
	Processes []Process `json:"processes"`
}

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

// ReturnResponse is used to return a response from server
func (s *Server) ReturnResponse(ctx context.Context, w http.ResponseWriter) {
	// todo: bad naming.
	log.Infof("Returning response with %d kimo processes...\n", len(s.Processes))
	w.Header().Set("Content-Type", "application/json")

	response := &Response{
		Processes: s.Processes,
	}
	json.NewEncoder(w).Encode(response)
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

// Procs is a handler for returning process list
func (s *Server) Procs(w http.ResponseWriter, req *http.Request) {
	forceParam := req.URL.Query().Get("force")
	fetch := false
	if forceParam == "true" || len(s.Processes) == 0 {
		fetch = true
	}

	if fetch {
		s.FetchAll()
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "access-control-allow-origin, access-control-allow-headers")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.ReturnResponse(ctx, w)

}

// Health is a dummy endpoint for load balancer health check
func (s *Server) Health(w http.ResponseWriter, req *http.Request) {
	// todo: real health check
	fmt.Fprintf(w, "OK")
}

// todo: bad naming.
func (s *Server) pollMetrics() {
	// todo: bad naming.
	s.PrometheusMetric.PollMetrics()
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

// Static serves static files (web components).
func (s *Server) Static() http.Handler {
	statikFS, err := fs.New()
	if err != nil {
		log.Errorln(err)
	}
	return http.FileServer(statikFS)

}

// Metrics is used to expose metrics that is compatible with Prometheus exporter
func (s *Server) Metrics() http.Handler {
	if len(s.Processes) == 0 {
		log.Debugln("Processes are not initialized. Polling...")
		s.PrometheusMetric.SetMetrics()
	}

	return promhttp.Handler()
}

// Run is used to run http handlers
func (s *Server) Run() error {
	// todo: move background jobs to another file. Keep only http related ones, here.
	// todo: reconsider context usages
	log.Infof("Running server on %s \n", s.Config.ListenAddress)

	go s.pollAgents()
	go s.pollMetrics()

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
