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
	s.KimoProcessChan = make(chan types.KimoProcess)
	return s
}

type Server struct {
	Config          *config.Server
	Mysql           *mysql.Mysql
	TcpProxy        *tcpproxy.TcpProxy
	KimoProcessChan chan types.KimoProcess
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
func (s *Server) Run() error {
	var wg sync.WaitGroup

	wg.Add(1)
	go s.getMysqlProcesses(&wg)

	wg.Add(1)
	go s.getTcpProxyRecords(&wg)

	wg.Wait()

	// get server info
	for _, mp := range s.Mysql.Processes {
		fmt.Printf("mp: %+v\n", mp)
		var kp types.KimoProcess
		kp.MysqlProcess = &mp
		// todo: debug log
		go s.GetDaemonProcess(&kp, mp.Host, mp.Port)
	}
	<-s.KimoProcessChan

	for kp := range s.KimoProcessChan {
		fmt.Printf("final kp: %+v\n", kp)
		fmt.Printf("final sp: %+v\n", kp.DaemonProcess)
		fmt.Printf("final tp: %+v\n", kp.TcpProxyProcess)
		fmt.Printf("final mp: %+v\n", kp.MysqlProcess)
		fmt.Printf("final tcp: %+v\n", kp.TcpProxyRecord)
	}

	return nil
}

func (s *Server) GetDaemonProcess(kp *types.KimoProcess, host string, port uint32) error {
	// todo: host validation
	// todo: use request with context
	var httpClient = &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://%s:%d/conns?ports=%d", host, s.Config.DaemonPort, port)
	fmt.Println("Requesting to ", url)
	response, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		fmt.Printf("Error: %s\n", response.Status)
		// todo: return appropriate error
		return errors.New("status code is not 200")
	}

	ksr := types.KimoDaemonResponse{}
	err = json.NewDecoder(response.Body).Decode(&ksr)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	// todo: do not return list from server
	dp := ksr.DaemonProcesses[0]
	dp.Hostname = ksr.Hostname

	if dp.Laddr.Port != port {
		return errors.New("could not found")
	}

	if dp.Name != "tcpproxy" {
		kp.DaemonProcess = &dp
		s.KimoProcessChan <- *kp
		return nil
	}

	kp.TcpProxyProcess = &dp
	pr, err := s.TcpProxy.GetProxyRecord(*kp.TcpProxyProcess, s.TcpProxy.Records)
	if err != nil {
		fmt.Println(err.Error())
		s.KimoProcessChan <- *kp
		return err
	}
	kp.TcpProxyRecord = pr
	err = s.GetDaemonProcess(kp, pr.ClientOutput.IP, kp.TcpProxyRecord.ClientOutput.Port)
	if err != nil {
		fmt.Println(err.Error())
		s.KimoProcessChan <- *kp
		return err
	}
	return nil
}
