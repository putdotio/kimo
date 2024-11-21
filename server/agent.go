package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cenkalti/log"
)

// AgentClient is agent client to fetch agent process from a kimo agent
type AgentClient struct {
	Host string
	Port uint32
}

type AgentError struct {
	Hostname string
	Status   string
}

type AgentResponse struct {
	Pid              int
	Name             string
	CmdLine          string
	Hostname         string
	ConnectionStatus string
}

func (ae *AgentError) Error() string {
	return fmt.Sprintf("Agent error. Host: %s - status: %s\n", ae.Hostname, ae.Status)
}

// NewAgentClient is constructor function for creating Agent object
func NewAgentClient(address IPPort) *AgentClient {
	ac := new(AgentClient)
	ac.Host = address.IP
	ac.Port = address.Port
	return ac
}

// Fetch is used to fetch agent process
func (ac *AgentClient) Get(ctx context.Context, port uint32) (*AgentResponse, error) {
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
	hostname := response.Header.Get("X-Kimo-Hostname")
	if response.StatusCode != 200 {
		return nil, &AgentError{Hostname: hostname, Status: response.Status}
	}

	type result struct {
		Status  string `json:"status"`
		Pid     int32  `json:"pid"`
		Name    string `json:"name"`
		CmdLine string `json:"cmdline"`
	}
	r := result{}
	err = json.NewDecoder(response.Body).Decode(&r)

	if err != nil {
		log.Errorln(err.Error())
		return nil, err
	}

	return &AgentResponse{
			ConnectionStatus: r.Status,
			Pid:              int(r.Pid),
			Name:             r.Name,
			CmdLine:          r.CmdLine,
			Hostname:         hostname},
		nil
}
