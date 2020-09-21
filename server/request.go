package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"kimo/mysql"
	"kimo/tcpproxy"
	"kimo/types"
	"net/http"
	"sync"
	"time"

	"github.com/cenkalti/log"
)

type KimoProcess struct {
	DaemonProcess  *types.DaemonProcess
	MysqlProcess   *types.MysqlProcess
	TcpProxyRecord *types.TcpProxyRecord
}

type KimoRequest struct {
	Mysql         *mysql.Mysql
	Server        *Server
	TcpProxy      *tcpproxy.TcpProxy
	DaemonPort    uint32
	KimoProcesses []*KimoProcess
}

func (s *Server) NewKimoRequest() *KimoRequest {
	kr := new(KimoRequest)
	kr.Mysql = mysql.NewMysql(s.Config.DSN)
	kr.TcpProxy = tcpproxy.NewTcpProxy(s.Config.TcpProxyMgmtAddress)
	kr.DaemonPort = s.Config.DaemonPort
	kr.KimoProcesses = make([]*KimoProcess, 0)
	kr.Server = s
	return kr
}

func (kr *KimoRequest) InitializeKimoProcesses(mps []*types.MysqlProcess, tprs []*types.TcpProxyRecord) error {
	for _, mp := range mps {
		tpr, err := kr.GetTcpProxyRecord(mp.Address, tprs)
		if err != nil {
			log.Debug(err.Error())
			// todo: handle
		}
		kr.KimoProcesses = append(kr.KimoProcesses, &KimoProcess{
			MysqlProcess:   mp,
			TcpProxyRecord: tpr,
		})
	}
	return nil
}

func (kr *KimoRequest) GetTcpProxyRecord(addr types.Addr, proxyRecords []*types.TcpProxyRecord) (*types.TcpProxyRecord, error) {
	for _, pr := range proxyRecords {
		if pr.ProxyOutput.Host == addr.Host && pr.ProxyOutput.Port == addr.Port {
			return pr, nil
		}
	}
	return nil, errors.New("could not found")
}

func (kr *KimoRequest) Setup(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errC := make(chan error)

	mysqlProcsC := make(chan []*types.MysqlProcess)
	proxyRecordsC := make(chan []*types.TcpProxyRecord)

	var mysqlProcesses []*types.MysqlProcess
	var tcpProxyRecords []*types.TcpProxyRecord

	go kr.Mysql.GetProcesses(ctx, mysqlProcsC, errC)
	go kr.TcpProxy.GetRecords(ctx, proxyRecordsC, errC)
	for {
		if mysqlProcesses != nil && tcpProxyRecords != nil {
			return kr.InitializeKimoProcesses(mysqlProcesses, tcpProxyRecords)
		}
		select {
		case mps := <-mysqlProcsC:
			mysqlProcesses = mps
		case tprs := <-proxyRecordsC:
			tcpProxyRecords = tprs
		case err := <-errC:
			cancel()
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

}
func (kr *KimoRequest) GetDaemonProcess(host string, port uint32) (*types.DaemonProcess, error) {
	// todo: use request with context
	var httpClient = &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://%s:%d/conns?ports=%d", host, kr.Server.Config.DaemonPort, port)
	log.Debugf("Requesting to %s\n", url)
	response, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		log.Errorf("Error: %s\n", response.Status)
		return nil, errors.New("status code is not 200")
	}

	ksr := types.KimoDaemonResponse{}
	err = json.NewDecoder(response.Body).Decode(&ksr)
	if err != nil {
		log.Errorln(err.Error())
		return nil, err
	}

	// todo: do not return list from server
	dp := ksr.DaemonProcesses[0]
	dp.Hostname = ksr.Hostname

	if dp.Laddr.Port != port {
		return nil, errors.New("could not found")
	}
	return &dp, nil
}
func (kr *KimoRequest) SetDaemonProcess(wg *sync.WaitGroup, kp *KimoProcess) {
	defer wg.Done()
	var host string
	var port uint32

	if kp.TcpProxyRecord != nil {
		host = kp.TcpProxyRecord.ClientOutput.Host
		port = kp.TcpProxyRecord.ClientOutput.Port
	} else {
		host = kp.MysqlProcess.Address.Host
		port = kp.MysqlProcess.Address.Port
	}
	dp, err := kr.GetDaemonProcess(host, port)
	if err != nil {
		kp.DaemonProcess = &types.DaemonProcess{}
	} else {
		kp.DaemonProcess = dp
	}
}
func (kr *KimoRequest) GenerateKimoProcesses(ctx context.Context) {
	doneC := make(chan bool)

	var wg sync.WaitGroup
	go func() {
		for _, kp := range kr.KimoProcesses {
			wg.Add(1)
			go kr.SetDaemonProcess(&wg, kp)
		}
		wg.Wait()
		doneC <- true
	}()

	for {
		select {
		case <-doneC:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (kr *KimoRequest) ReturnResponse(ctx context.Context, w http.ResponseWriter) {
	serverProcesses := make([]types.ServerProcess, 0)
	for _, kp := range kr.KimoProcesses {
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
