package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"kimo/types"
	"net/http"

	"github.com/cenkalti/log"
)

// AgentClient is agent client to fetch agent process from a kimo agent
type AgentClient struct {
	Host string
	Port uint32
}

// NewAgentClient is constructor function for creating Agent object
func NewAgentClient(host string, port uint32) *AgentClient {
	a := new(AgentClient)
	a.Host = host
	a.Port = port
	return a
}

// Fetch is used to fetch agent process
func (ac *AgentClient) Get(ctx context.Context, port uint32) (*types.AgentProcess, error) {
	url := fmt.Sprintf("http://%s:%d/proc?port=%d", ac.Host, ac.Port, port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	log.Debugf("Requesting to %s\n", url)
	response, err := client.Do(req)
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
