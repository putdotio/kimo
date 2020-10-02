package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"kimo/types"
	"net/http"
	"sync"
	"time"

	"github.com/cenkalti/log"
)

type KimoProcess struct {
	DaemonProcess  *types.DaemonProcess
	MysqlProcess   *types.MysqlProcess
	TCPProxyRecord *types.TCPProxyRecord
	KimoRequest    *KimoRequest
	Logger         log.Logger
}

func (kp *KimoProcess) FetchDaemonProcess(ctx context.Context, host string, port uint32) (*types.DaemonProcess, error) {
	// todo: use request with context
	var httpClient = &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://%s:%d/conns?port=%d", host, kp.KimoRequest.Server.Config.DaemonPort, port)
	kp.Logger.Debugf("Requesting to %s\n", url)
	response, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		kp.Logger.Errorf("Error: %s\n", response.Status)
		return nil, errors.New("status code is not 200")
	}

	dp := types.DaemonProcess{}
	err = json.NewDecoder(response.Body).Decode(&dp)

	// todo: consider NotFound
	if err != nil {
		kp.Logger.Errorln(err.Error())
		return nil, err
	}

	return &dp, nil
}

func (kp *KimoProcess) SetDaemonProcess(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	var host string
	var port uint32

	if kp.TCPProxyRecord != nil {
		host = kp.TCPProxyRecord.ClientOutput.Host
		port = kp.TCPProxyRecord.ClientOutput.Port
	} else {
		host = kp.MysqlProcess.Address.Host
		port = kp.MysqlProcess.Address.Port
	}
	dp, err := kp.FetchDaemonProcess(ctx, host, port)
	if err != nil {
		kp.DaemonProcess = &types.DaemonProcess{}
	} else {
		kp.DaemonProcess = dp
	}
}
