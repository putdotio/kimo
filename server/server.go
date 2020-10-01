package server

import (
	"context"
	"kimo/config"
	"net/http"
	"time"

	"github.com/cenkalti/log"
	_ "github.com/go-sql-driver/mysql"
)

func NewServer(cfg *config.Server) *Server {
	log.Infoln("Creating a new server...")
	s := new(Server)
	s.Config = cfg
	return s
}

type Server struct {
	Config *config.Server
}

func (s *Server) Processes(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	kr := s.NewKimoRequest()
	log.Infoln("Setup...")
	err := kr.Setup(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Infoln("Generating kimo processes...")
	kr.GenerateKimoProcesses(ctx)
	log.Infoln("Returning response...")
	kr.ReturnResponse(ctx, w)

}

func (s *Server) Run() error {
	log.Infof("Running server on %s \n", s.Config.ListenAddress)
	http.HandleFunc("/procs", s.Processes)
	err := http.ListenAndServe(s.Config.ListenAddress, nil)
	if err != nil {
		log.Errorln(err.Error())
		return err
	}
	return nil
}
