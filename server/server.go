package server

import (
	"context"
	"fmt"
	"kimo/config"
	"net/http"
	"time"

	"github.com/cenkalti/log"
	"github.com/rakyll/statik/fs"

	_ "kimo/statik" // Auto-generated module by statik.
)

// NewServer is used to create a new Server type
func NewServer(cfg *config.Config) *Server {
	s := new(Server)
	s.Config = &cfg.Server
	log.Infoln("Creating a new server...")
	return s
}

// Server is a type for handling server side
type Server struct {
	Config *config.Server
}

// Processes is a handler for returning process list
func (s *Server) Processes(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "access-control-allow-origin, access-control-allow-headers")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	kr := s.NewKimoRequest()
	err := kr.Setup(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	kr.GenerateKimoProcesses(ctx)
	kr.ReturnResponse(ctx, w)

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

// Run is used to run http handlers
func (s *Server) Run() error {
	log.Infof("Running server on %s \n", s.Config.ListenAddress)

	http.Handle("/", s.Static())
	http.HandleFunc("/procs", s.Processes)
	http.HandleFunc("/health", s.Health)
	err := http.ListenAndServe(s.Config.ListenAddress, nil)
	if err != nil {
		log.Errorln(err.Error())
		return err
	}
	return nil
}
