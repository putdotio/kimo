package server

import (
	"context"
	"encoding/json"
	"kimo/mysql"
	"kimo/tcpproxy"
	"kimo/types"
	"net/http"
	"strconv"
	"sync"

	"github.com/cenkalti/log"
)

type KimoRequest struct {
	Mysql         *mysql.Mysql
	Server        *Server
	TCPProxy      *tcpproxy.TCPProxy
	DaemonPort    uint32
	KimoProcesses []*KimoProcess
	Logger        log.Logger
}

// todo: reconsider logger usages
func (s *Server) NewKimoRequest() *KimoRequest {
	kr := new(KimoRequest)
	kr.Mysql = mysql.NewMysql(s.Config.DSN)
	kr.Mysql.Logger = s.Logger
	kr.TCPProxy = tcpproxy.NewTCPProxy(s.Config.TCPProxyMgmtAddress)
	kr.TCPProxy.Logger = s.Logger
	kr.DaemonPort = s.Config.DaemonPort
	kr.KimoProcesses = make([]*KimoProcess, 0)
	kr.Server = s
	kr.Logger = s.Logger
	return kr
}

func (kr *KimoRequest) InitializeKimoProcesses(mps []*types.MysqlProcess, tprs []*types.TCPProxyRecord) error {
	for _, mp := range mps {
		tpr := kr.FetchTCPProxyRecord(mp.Address, tprs)
		if tpr == nil {
			continue
		}
		kr.KimoProcesses = append(kr.KimoProcesses, &KimoProcess{
			MysqlProcess:   mp,
			TCPProxyRecord: tpr,
			KimoRequest:    kr,
			Logger:         kr.Logger,
		})
	}
	return nil
}

func (kr *KimoRequest) FetchTCPProxyRecord(addr types.Addr, proxyRecords []*types.TCPProxyRecord) *types.TCPProxyRecord {
	for _, pr := range proxyRecords {
		if pr.ProxyOutput.Host == addr.Host && pr.ProxyOutput.Port == addr.Port {
			return pr
		}
	}
	return nil
}

func (kr *KimoRequest) Setup(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errC := make(chan error)

	mysqlProcsC := make(chan []*types.MysqlProcess)
	proxyRecordsC := make(chan []*types.TCPProxyRecord)

	var mysqlProcesses []*types.MysqlProcess
	var tcpProxyRecords []*types.TCPProxyRecord

	go kr.Mysql.FetchProcesses(ctx, mysqlProcsC, errC)
	go kr.TCPProxy.FetchRecords(ctx, proxyRecordsC, errC)
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
			kr.Logger.Errorf("Error occured: %s", err.Error())
			cancel()
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

}

func (kr *KimoRequest) GenerateKimoProcesses(ctx context.Context) {
	var wg sync.WaitGroup
	for _, kp := range kr.KimoProcesses {
		wg.Add(1)
		go kp.SetDaemonProcess(ctx, &wg)
	}
	wg.Wait()
}

func (kr *KimoRequest) ReturnResponse(ctx context.Context, w http.ResponseWriter) {
	serverProcesses := make([]types.ServerProcess, 0)
	for _, kp := range kr.KimoProcesses {

		ut, err := strconv.ParseUint(kp.MysqlProcess.Time, 10, 32)
		if err != nil {
			kr.Logger.Errorf("time %s could not be converted to int", kp.MysqlProcess.Time)
		}
		t := uint32(ut)

		serverProcesses = append(serverProcesses, types.ServerProcess{
			ID:        kp.MysqlProcess.ID,
			MysqlUser: kp.MysqlProcess.User,
			DB:        kp.MysqlProcess.DB.String,
			Command:   kp.MysqlProcess.Command,
			Time:      t,
			State:     kp.MysqlProcess.State.String,
			Info:      kp.MysqlProcess.Info.String,
			CmdLine:   kp.DaemonProcess.CmdLine,
			Pid:       kp.DaemonProcess.Pid,
			Host:      kp.DaemonProcess.Hostname,
		})
	}

	w.Header().Set("Content-Type", "application/json")

	response := &types.KimoServerResponse{
		ServerProcesses: serverProcesses,
	}
	json.NewEncoder(w).Encode(response)
}
