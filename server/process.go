package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"kimo/types"
	"sync"
	"time"

	"github.com/cenkalti/log"
)

type KimoProcess struct {
	DaemonProcess  *types.DaemonProcess
	MysqlProcess   *MysqlProcess
	TCPProxyRecord *TCPProxyRecord
	KimoRequest    *KimoRequest
	Server         *Server
}

func (kp *KimoProcess) FetchDaemonProcess(ctx context.Context, host string, port uint32) (*types.DaemonProcess, error) {
	// todo: use request with context
	var httpClient = NewHttpClient(kp.Server.Config.DaemonConnectTimeout*time.Second, kp.Server.Config.DaemonReadTimeout*time.Second)
	url := fmt.Sprintf("http://%s:%d/proc?port=%d", host, kp.KimoRequest.Server.Config.DaemonPort, port)
	log.Debugf("Requesting to %s\n", url)
	response, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		log.Debugf("Error: %s -> %s\n", url, response.Status)
		return nil, errors.New("status code is not 200")
	}

	dp := types.DaemonProcess{}
	err = json.NewDecoder(response.Body).Decode(&dp)

	// todo: consider NotFound
	if err != nil {
		log.Errorln(err.Error())
		return nil, err
	}

	return &dp, nil
}

func (kp *KimoProcess) SetDaemonProcess(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	var host string
	var port uint32

	if kp.TCPProxyRecord != nil {
		host = kp.TCPProxyRecord.ClientOut.IP
		port = kp.TCPProxyRecord.ClientOut.Port
	} else {
		host = kp.MysqlProcess.Address.IP
		port = kp.MysqlProcess.Address.Port
	}
	dp, err := kp.FetchDaemonProcess(ctx, host, port)
	if err != nil {
		kp.DaemonProcess = &types.DaemonProcess{}
	} else {
		kp.DaemonProcess = dp
	}
}
