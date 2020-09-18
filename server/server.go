package server

import (
	"context"
	"kimo/config"
	"net/http"

	"github.com/cenkalti/log"
	_ "github.com/go-sql-driver/mysql"
)

func NewServer(cfg *config.Server) *Server {
	s := new(Server)
	s.Config = cfg
	return s
}

type Server struct {
	Config *config.Server
}

func (s *Server) Processes(w http.ResponseWriter, req *http.Request) {
	// todo: error handling
	// todo: context
	ctx := context.Background()
	kr := s.NewKimoRequest(ctx)
	log.Infoln("Setup...")
	kr.Setup()
	log.Infoln("Generating kimo processes...")
	kps := kr.GenerateKimoProcesses()
	log.Infoln("Returning response...")
	kr.ReturnResponse(w, kps)

}

func (s *Server) Run() error {
	http.HandleFunc("/procs", s.Processes)
	err := http.ListenAndServe(s.Config.ListenAddress, nil)
	if err != nil {
		return err
		// todo: handle error
	}
	return nil
}
