package server

import (
	"encoding/json"
	"kimo/config"
	"kimo/mysql"
	"kimo/tcpproxy"
	"kimo/types"
	"net/http"
	"sync"

	"github.com/cenkalti/log"
	_ "github.com/go-sql-driver/mysql"
)

func NewServer(cfg *config.Server) *Server {
	s := new(Server)
	s.Config = cfg
	s.Mysql = mysql.NewMysql(cfg.DSN)
	s.TcpProxy = tcpproxy.NewTcpProxy(cfg.TcpProxyMgmtAddress)
	return s
}

type Server struct {
	Config   *config.Server
	Mysql    *mysql.Mysql
	TcpProxy *tcpproxy.TcpProxy
}

func (s *Server) getMysqlProcesses(wg *sync.WaitGroup) error {
	defer wg.Done()
	// todo: use context
	err := s.Mysql.GetProcesses()
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) getTcpProxyRecords(wg *sync.WaitGroup) error {
	defer wg.Done()

	// todo: use context
	err := s.TcpProxy.GetRecords()
	if err != nil {
		return err
	}
	return nil
}

func (s *Server) Setup() {
	var wg sync.WaitGroup

	wg.Add(1)
	go s.getMysqlProcesses(&wg)

	wg.Add(1)
	go s.getTcpProxyRecords(&wg)

	wg.Wait()
}

func (s *Server) GenerateKimoProcesses() []KimoProcess {
	kChan := make(chan KimoProcess)

	// get server info
	var wg sync.WaitGroup
	go func() {
		for _, mp := range s.Mysql.Processes {
			var kp KimoProcess
			kp.Server = s
			kp.MysqlProcess = &mp
			wg.Add(1)
			kp.SetDaemonProcess(&wg, kChan)
		}
		wg.Wait()
		close(kChan)
	}()

	kps := make([]KimoProcess, 0)
	for {
		kp, ok := <-kChan
		if !ok {
			break
		}
		kps = append(kps, kp)
	}
	return kps
}

func (s *Server) ReturnResponse(w http.ResponseWriter, kps []KimoProcess) {
	serverProcesses := make([]types.ServerProcess, 0)
	for _, kp := range kps {
		serverProcesses = append(serverProcesses, types.ServerProcess{
			ID:        kp.MysqlProcess.ID,
			MysqlUser: kp.MysqlProcess.User,
			DB:        kp.MysqlProcess.DB.String,
			Command:   kp.MysqlProcess.Command,
			Time:      kp.MysqlProcess.Time,
			State:     kp.MysqlProcess.State.String,
			Info:      kp.MysqlProcess.Info.String,
			CmdLine:   kp.DaemonProcess.CmdLine,
			Pid:       kp.DaemonProcess.Pid,
		})
	}

	w.Header().Set("Content-Type", "application/json")

	response := &types.KimoServerResponse{
		ServerProcesses: serverProcesses,
	}
	json.NewEncoder(w).Encode(response)
}
func (s *Server) Processes(w http.ResponseWriter, req *http.Request) {
	// todo: error handling
	// todo: debug log
	log.Infoln("Setup...")
	s.Setup()
	log.Infoln("Generating kimo processes...")
	kps := s.GenerateKimoProcesses()
	log.Infoln("Returning response...")
	s.ReturnResponse(w, kps)

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
