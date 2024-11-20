package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cenkalti/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rakyll/statik/fs"

	_ "kimo/statik" // Auto-generated module by statik.
)

// Response is type for returning a response from kimo server
type Response struct {
	Processes []KimoProcess `json:"processes"`
}

// Procs is a handler for returning process list
func (s *Server) Procs(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "access-control-allow-origin, access-control-allow-headers")
	w.Header().Set("Content-Type", "application/json")

	response := &Response{
		Processes: s.KimoProcesses,
	}
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, "Can not encode process", http.StatusInternalServerError)
	}

}

// Health is a dummy endpoint for load balancer health check
func (s *Server) Health(w http.ResponseWriter, req *http.Request) {
	// todo: real health check
	fmt.Fprintf(w, "OK")
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
	return promhttp.Handler()
}

// Run is used to run http handlers
func (s *Server) Run() error {
	// todo: reconsider context usages
	log.Infof("Running server on %s \n", s.Config.ListenAddress)

	go s.pollAgents()

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
