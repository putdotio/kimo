package server

import (
	"context"
	"encoding/json"
	"kimo/mysql"
	"kimo/tcpproxy"
	"kimo/types"
	"net/http"
	"sync"

	"github.com/cenkalti/log"
)

func (s *Server) NewKimoRequest() *KimoRequest {
	kr := new(KimoRequest)
	kr.Mysql = mysql.NewMysql(s.Config.DSN)
	kr.TcpProxy = tcpproxy.NewTcpProxy(s.Config.TcpProxyMgmtAddress)
	kr.DaemonPort = s.Config.DaemonPort
	kr.ErrChan = make(chan error)
	return kr
}

type KimoRequest struct {
	Mysql      *mysql.Mysql
	TcpProxy   *tcpproxy.TcpProxy
	DaemonPort uint32
	ErrChan    chan error
}

func (kr *KimoRequest) Setup(ctx context.Context) error {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := kr.Mysql.GetProcesses(ctx)
		if err != nil {
			log.Errorln(err.Error())
			kr.ErrChan <- err
			return
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := kr.TcpProxy.GetRecords(ctx)
		if err != nil {
			log.Errorln(err.Error())
			kr.ErrChan <- err
			return
		}
	}()

	wg.Wait()
	close(kr.ErrChan)

	select {
	case err := <-kr.ErrChan:
		return err
	default:
	}
	return nil
}

func (kr *KimoRequest) GenerateKimoProcesses(ctx context.Context) []*KimoProcess {
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
