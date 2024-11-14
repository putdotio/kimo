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

type NotFoundError struct {
	Host string
}

func (n *NotFoundError) Error() string {
	return fmt.Sprintf("Host %s returned 404\n", n.Host)
}

// NewAgentClient is constructor function for creating Agent object
func NewAgentClient(host string, port uint32) *AgentClient {
	ac := new(AgentClient)
	ac.Host = host
	ac.Port = port
	return ac
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
		if response.StatusCode == 404 {
			hostname := response.Header.Get("X-Hostname")
			if hostname != "" {
				return nil, &NotFoundError{Host: hostname}
			}
		}
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
