package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"kimo/config"
	"kimo/mysql"
	"kimo/tcpproxy"
	"kimo/types"
	"net/http"
	"sync"
	"time"

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

func (s *Server) GetDaemonProcesses() []KimoProcess {
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
	s.Setup()
	kps := s.GetDaemonProcesses()
	s.ReturnResponse(w, kps)

}

type KimoProcess struct {
	Server          *Server
	DaemonProcess   *types.DaemonProcess
	TcpProxyProcess *types.DaemonProcess
	MysqlProcess    *types.MysqlProcess
	TcpProxyRecord  *types.TcpProxyRecord
}

func (kp *KimoProcess) SetDaemonProcess(wg *sync.WaitGroup, kChan chan<- KimoProcess) {
	defer wg.Done()
	dp, err := kp.GetDaemonProcess(kp.MysqlProcess.Host, kp.MysqlProcess.Port)
	if err != nil {
		kp.DaemonProcess = &types.DaemonProcess{}
		kChan <- *kp
		return
	}
	kp.DaemonProcess = dp
	kChan <- *kp
}

func (kp *KimoProcess) GetDaemonProcess(host string, port uint32) (*types.DaemonProcess, error) {
	// todo: host validation
	// todo: use request with context
	var httpClient = &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://%s:%d/conns?ports=%d", host, kp.Server.Config.DaemonPort, port)
	fmt.Println("Requesting to ", url)
	response, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		fmt.Printf("Error: %s\n", response.Status)
		// todo: return appropriate error
		return nil, errors.New("status code is not 200")
	}

	ksr := types.KimoDaemonResponse{}
	err = json.NewDecoder(response.Body).Decode(&ksr)
	if err != nil {
		fmt.Println(err.Error())
		kp.DaemonProcess = &types.DaemonProcess{}
		return nil, err
	}

	// todo: do not return list from server
	dp := ksr.DaemonProcesses[0]
	dp.Hostname = ksr.Hostname

	if dp.Laddr.Port != port {
		kp.DaemonProcess = &types.DaemonProcess{}
		return nil, errors.New("could not found")
	}

	if dp.Name != "tcpproxy" {
		return &dp, nil
	}

	kp.TcpProxyProcess = &dp
	pr, err := kp.Server.TcpProxy.GetProxyRecord(dp, kp.Server.TcpProxy.Records)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	kp.TcpProxyRecord = pr
	return kp.GetDaemonProcess(pr.ClientOutput.IP, kp.TcpProxyRecord.ClientOutput.Port)
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
