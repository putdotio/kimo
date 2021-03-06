package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"kimo/config"
	"kimo/types"
	"time"

	"github.com/cenkalti/log"
)

// Agent is agent client to fetch agent process from Kimo Agent
type Agent struct {
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	Port           uint32
}

// NewAgent is constructor function for creating Agent object
func NewAgent(cfg config.Server) *Agent {
	a := new(Agent)
	a.Port = cfg.AgentPort
	a.ConnectTimeout = cfg.AgentConnectTimeout
	a.ReadTimeout = cfg.AgentReadTimeout
	return a
}

// Fetch is used to fetch agent process
func (a *Agent) Fetch(ctx context.Context, host string, port uint32) (*types.AgentProcess, error) {
	// todo: use request with context
	var httpClient = NewHTTPClient(a.ConnectTimeout*time.Second, a.ReadTimeout*time.Second)
	url := fmt.Sprintf("http://%s:%d/proc?port=%d", host, a.Port, port)
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

	ap := types.AgentProcess{}
	err = json.NewDecoder(response.Body).Decode(&ap)

	// todo: consider NotFound
	if err != nil {
		log.Errorln(err.Error())
		return nil, err
	}

	return &ap, nil
}
