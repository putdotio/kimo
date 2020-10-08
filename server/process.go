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

// KimoProcess is combined with processes from mysql to agent through tcpproxy
type KimoProcess struct {
	AgentProcess   *types.AgentProcess
	MysqlProcess   *MysqlProcess
	TCPProxyRecord *TCPProxyRecord
	Server         *Server
}

// FetchAgentProcess is used to fetch process information an agent
func (kp *KimoProcess) FetchAgentProcess(ctx context.Context, host string, port uint32) (*types.AgentProcess, error) {
	// todo: use request with context
	var httpClient = NewHTTPClient(kp.Server.Config.AgentConnectTimeout*time.Second, kp.Server.Config.AgentReadTimeout*time.Second)
	url := fmt.Sprintf("http://%s:%d/proc?port=%d", host, kp.Server.Config.AgentPort, port)
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

	dp := types.AgentProcess{}
	err = json.NewDecoder(response.Body).Decode(&dp)

	// todo: consider NotFound
	if err != nil {
		log.Errorln(err.Error())
		return nil, err
	}

	return &dp, nil
}

// SetAgentProcess is used to set agent process of a KimoProcess
func (kp *KimoProcess) SetAgentProcess(ctx context.Context, wg *sync.WaitGroup) {
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
	dp, err := kp.FetchAgentProcess(ctx, host, port)
	if err != nil {
		kp.AgentProcess = &types.AgentProcess{}
	} else {
		kp.AgentProcess = dp
	}
}
