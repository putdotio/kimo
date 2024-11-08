package server

import (
	"encoding/json"
	"fmt"
	"net"
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

// NewHTTPClient returns a http client with custom connect & read timeout
func NewHTTPClient(connectTimeout, readTimeout time.Duration) *http.Client {
	return &http.Client{
		// Set total timeout slightly higher than sum of connect + read
		Timeout: (connectTimeout + readTimeout) + 2*time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: connectTimeout * time.Second,
			}).DialContext,
			IdleConnTimeout: 90 * time.Second,
		},
	}
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

	log.Infof("Returning response with %d kimo processes...\n", len(s.Processes))
	w.Header().Set("Content-Type", "application/json")

	response := &Response{
		Processes: s.Processes,
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
	// todo: separate prometheus and json metrics
	return promhttp.Handler()
}

// Run is used to run http handlers
func (s *Server) Run() error {
	// todo: reconsider context usages
	log.Infof("Running server on %s \n", s.Config.ListenAddress)

	go s.pollAgents()
	go s.setMetrics()

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
