package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"kimo/types"
	"net/http"
	"sync"
	"time"
)

type KimoProcess struct {
	Server          *Server
	DaemonProcess   *types.DaemonProcess
	TcpProxyProcess *types.DaemonProcess
	MysqlProcess    *types.MysqlProcess
	TcpProxyRecord  *types.TcpProxyRecord
}

func (kp *KimoProcess) SetDaemonProcess(wg *sync.WaitGroup, kChan chan<- KimoProcess) {
	defer wg.Done()
	dp, err := kp.GetDaemonProcess(kp.MysqlProcess.Host, kp.MysqlProcess.Port)
	if err != nil {
		kp.DaemonProcess = &types.DaemonProcess{}
		kChan <- *kp
		return
	}
	kp.DaemonProcess = dp
	kChan <- *kp
}

func (kp *KimoProcess) GetDaemonProcess(host string, port uint32) (*types.DaemonProcess, error) {
	// todo: host validation
	// todo: use request with context
	var httpClient = &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://%s:%d/conns?ports=%d", host, kp.Server.Config.DaemonPort, port)
	fmt.Println("Requesting to ", url)
	response, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		fmt.Printf("Error: %s\n", response.Status)
		// todo: return appropriate error
		return nil, errors.New("status code is not 200")
	}

	ksr := types.KimoDaemonResponse{}
	err = json.NewDecoder(response.Body).Decode(&ksr)
	if err != nil {
		fmt.Println(err.Error())
		kp.DaemonProcess = &types.DaemonProcess{}
		return nil, err
	}

	// todo: do not return list from server
	dp := ksr.DaemonProcesses[0]
	dp.Hostname = ksr.Hostname

	if dp.Laddr.Port != port {
		kp.DaemonProcess = &types.DaemonProcess{}
		return nil, errors.New("could not found")
	}

	if dp.Name != "tcpproxy" {
		return &dp, nil
	}

	kp.TcpProxyProcess = &dp
	pr, err := kp.Server.TcpProxy.GetProxyRecord(dp, kp.Server.TcpProxy.Records)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	kp.TcpProxyRecord = pr
	return kp.GetDaemonProcess(pr.ClientOutput.IP, kp.TcpProxyRecord.ClientOutput.Port)
}
