package server

import (
	"context"
	"encoding/json"
	"kimo/mysql"
	"kimo/tcpproxy"
	"kimo/types"
	"net/http"
	"sync"
	"time"
)

func (s *Server) NewKimoRequest(ctx context.Context) *KimoRequest {
	kr := new(KimoRequest)
	kr.Mysql = mysql.NewMysql(s.Config.DSN)
	kr.TcpProxy = tcpproxy.NewTcpProxy(s.Config.TcpProxyMgmtAddress)
	kr.DaemonPort = s.Config.DaemonPort
	ctx, cancel := context.WithTimeout(ctx, time.Second*2)
	kr.Context = ctx
	kr.ContextCancel = cancel
	return kr
}

type KimoRequest struct {
	Mysql         *mysql.Mysql
	TcpProxy      *tcpproxy.TcpProxy
	Context       context.Context
	DaemonPort    uint32
	ContextCancel context.CancelFunc
}

func (kr *KimoRequest) getMysqlProcesses(wg *sync.WaitGroup) error {
	defer wg.Done()
	// todo: use context
	err := kr.Mysql.GetProcesses()
	if err != nil {
		return err
	}
	return nil
}

func (kr *KimoRequest) getTcpProxyRecords(wg *sync.WaitGroup) error {
	defer wg.Done()

	// todo: use context
	err := kr.TcpProxy.GetRecords()
	if err != nil {
		return err
	}
	return nil
}

func (kr *KimoRequest) Setup() {
	var wg sync.WaitGroup

	wg.Add(1)
	go kr.getMysqlProcesses(&wg)

	wg.Add(1)
	go kr.getTcpProxyRecords(&wg)

	wg.Wait()
}

func (kr *KimoRequest) GenerateKimoProcesses() []*KimoProcess {
	kpChan := make(chan KimoProcess)

	// get server info
	var wg sync.WaitGroup
	go func() {
		for _, mp := range kr.Mysql.Processes {
			var kp KimoProcess
			kp.KimoRequest = kr
			kp.MysqlProcess = &mp
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
		case <-kr.Context.Done():
			break readChannel
		}
	}
	return kps
}

func (kr *KimoRequest) ReturnResponse(w http.ResponseWriter, kps []*KimoProcess) {
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
