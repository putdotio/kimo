package server

import (
	"encoding/json"
	"net/http"

	"github.com/cenkalti/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rakyll/statik/fs"

	_ "kimo/statik" // Auto-generated module by statik.
)

// Response contains basic process information for API responses.
type Response struct {
	Processes []KimoProcess `json:"processes"`
}

// Procs is a handler for returning process list
func (s *Server) Procs(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "access-control-allow-origin, access-control-allow-headers")
	w.Header().Set("Content-Type", "application/json")

	response := &Response{
		Processes: s.GetProcesses(),
	}
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, "Can not encode process", http.StatusInternalServerError)
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
	return promhttp.Handler()
}
