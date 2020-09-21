package server

import (
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
	TcpProxyRecord *types.TcpProxyRecord
	KimoRequest    *KimoRequest
}

func (kp *KimoProcess) GetDaemonProcess(host string, port uint32) (*types.DaemonProcess, error) {
	// todo: use request with context
	var httpClient = &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://%s:%d/conns?ports=%d", host, kp.KimoRequest.Server.Config.DaemonPort, port)
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

func (kp *KimoProcess) SetDaemonProcess(wg *sync.WaitGroup) {
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
	dp, err := kp.GetDaemonProcess(host, port)
	if err != nil {
		kp.DaemonProcess = &types.DaemonProcess{}
	} else {
		kp.DaemonProcess = dp
	}
}
