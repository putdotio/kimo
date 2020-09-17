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
func (s *Server) Processes(w http.ResponseWriter, req *http.Request) {
	// todo: debug log
	var wg sync.WaitGroup

	wg.Add(1)
	go s.getMysqlProcesses(&wg)

	wg.Add(1)
	go s.getTcpProxyRecords(&wg)

	wg.Wait()

	serverProcesses := make([]types.ServerProcess, 0)
	kChan := make(chan types.KimoProcess)

	// get server info
	go func() {
		for _, mp := range s.Mysql.Processes {
			var kp types.KimoProcess
			kp.MysqlProcess = &mp
			go s.GetDaemonProcess(&kp, kp.MysqlProcess.Host, kp.MysqlProcess.Port, kChan)
		}
	}()

	for {
		fmt.Printf("len of serverProcesses: %d\n", len(serverProcesses))
		fmt.Printf("len of mysqlProcesses: %d\n", len(s.Mysql.Processes))
		if len(serverProcesses) == len(s.Mysql.Processes) {
			break
		}

		kp := <-kChan
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

func (s *Server) Run() error {
	http.HandleFunc("/procs", s.Processes)
	err := http.ListenAndServe(s.Config.ListenAddress, nil)
	if err != nil {
		return err
		// todo: handle error
	}
	return nil
}

func (s *Server) GetDaemonProcess(kp *types.KimoProcess, host string, port uint32, kChan chan<- types.KimoProcess) error {
	// todo: return kp. do not send to channel.
	// todo: host validation
	// todo: use request with context
	var httpClient = &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://%s:%d/conns?ports=%d", host, s.Config.DaemonPort, port)
	fmt.Println("Requesting to ", url)
	response, err := httpClient.Get(url)
	if err != nil {
		kp.DaemonProcess = &types.DaemonProcess{}
		kChan <- *kp
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		fmt.Printf("Error: %s\n", response.Status)
		// todo: return appropriate error
		kp.DaemonProcess = &types.DaemonProcess{}
		kChan <- *kp
		return errors.New("status code is not 200")
	}

	ksr := types.KimoDaemonResponse{}
	err = json.NewDecoder(response.Body).Decode(&ksr)
	if err != nil {
		fmt.Println(err.Error())
		kp.DaemonProcess = &types.DaemonProcess{}
		kChan <- *kp
		return err
	}

	// todo: do not return list from server
	dp := ksr.DaemonProcesses[0]
	dp.Hostname = ksr.Hostname

	if dp.Laddr.Port != port {
		kp.DaemonProcess = &types.DaemonProcess{}
		kChan <- *kp
		return errors.New("could not found")
	}

	if dp.Name != "tcpproxy" {
		kp.DaemonProcess = &dp
		kChan <- *kp
		return nil
	}

	kp.TcpProxyProcess = &dp
	pr, err := s.TcpProxy.GetProxyRecord(*kp.TcpProxyProcess, s.TcpProxy.Records)
	if err != nil {
		fmt.Println(err.Error())
		kp.DaemonProcess = &types.DaemonProcess{}
		kChan <- *kp
		return err
	}
	kp.TcpProxyRecord = pr
	err = s.GetDaemonProcess(kp, pr.ClientOutput.IP, kp.TcpProxyRecord.ClientOutput.Port, kChan)
	if err != nil {
		fmt.Println(err.Error())
		kp.DaemonProcess = &types.DaemonProcess{}
		kChan <- *kp
		return err
	}
	kp.DaemonProcess = &dp
	kChan <- *kp
	return nil
}
