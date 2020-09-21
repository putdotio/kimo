package server

import (
	"context"
	"encoding/json"
	"kimo/mysql"
	"kimo/tcpproxy"
	"kimo/types"
	"net/http"
	"sync"
)

func (s *Server) NewKimoRequest() *KimoRequest {
	kr := new(KimoRequest)
	kr.Mysql = mysql.NewMysql(s.Config.DSN)
	kr.TcpProxy = tcpproxy.NewTcpProxy(s.Config.TcpProxyMgmtAddress)
	kr.DaemonPort = s.Config.DaemonPort
	return kr
}

type KimoRequest struct {
	Mysql           *mysql.Mysql
	TcpProxy        *tcpproxy.TcpProxy
	DaemonPort      uint32
	MysqlProcesses  []*types.MysqlProcess
	TcpProxyRecords []*types.TcpProxyRecord
}

func (kr *KimoRequest) Setup(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errC := make(chan error)

	mysqlProcsC := make(chan []*types.MysqlProcess)
	proxyRecordsC := make(chan []*types.TcpProxyRecord)

	go kr.Mysql.GetProcesses(ctx, mysqlProcsC, errC)
	go kr.TcpProxy.GetRecords(ctx, proxyRecordsC, errC)
	for {
		if kr.MysqlProcesses != nil && kr.TcpProxyRecords != nil {
			return nil
		}
		select {
		case mps := <-mysqlProcsC:
			kr.MysqlProcesses = mps
		case prs := <-proxyRecordsC:
			kr.TcpProxyRecords = prs
		case err := <-errC:
			cancel()
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

}

func (kr *KimoRequest) GenerateKimoProcesses(ctx context.Context) []*KimoProcess {
	kpChan := make(chan KimoProcess)

	// get server info
	var wg sync.WaitGroup
	go func() {
		for _, mp := range kr.MysqlProcesses {
			var kp KimoProcess
			kp.KimoRequest = kr
			kp.MysqlProcess = mp
			wg.Add(1)
			go kp.SetDaemonProcess(&wg, kpChan)
		}
		wg.Wait()
		close(kpChan)
	}()

	kps := make([]*KimoProcess, 0)

readChannel:
	for {
		select {
		case kp, ok := <-kpChan:
			if !ok {
				break readChannel
			}
			kps = append(kps, &kp)
		case <-ctx.Done():
			break readChannel
		}
	}
	return kps
}

func (kr *KimoRequest) ReturnResponse(ctx context.Context, w http.ResponseWriter, kps []*KimoProcess) {
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
