package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cenkalti/log"
)

// AgentProcess represents process info from a kimo-agent enhanced with response detail.
type AgentProcess struct {
	Pid              int
	Name             string
	Cmdline          string
	ConnectionStatus string
	Address          IPPort // kimo agent's address.
	hostname         string
	err              error
}

// NewAgentProcess creates and returns a new AgentProcess.
func NewAgentProcesss(ar *AgentResponse, address IPPort, err error) *AgentProcess {
	ap := new(AgentProcess)
	ap.err = err
	ap.Address = address

	if ar != nil {
		ap.Pid = ar.Pid
		ap.Name = ar.Name
		ap.Cmdline = ar.CmdLine
		ap.hostname = ar.Hostname
	}
	return ap
}

// Host is kimo agent's hostname if response is returned, otherwise host's IP.
func (ap *AgentProcess) Host() string {
	if ap.hostname != "" {
		return ap.hostname
	}
	if ap.err != nil {
		if aErr, ok := ap.err.(*AgentError); ok {
			return aErr.Hostname
		}
	}
	return ap.Address.IP
}

// AgentClient represents an agent client to fetch get process from a kimo-agent
type AgentClient struct {
	Host string
	Port uint32
}

// AgentError represents an HTTP error that is retured from kimo-agent.
type AgentError struct {
	Hostname string
	Status   string
}

// AgentResponse represents a success response from kimo-agent.
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

// NewAgentClient creates and returns a new AgentClient.
func NewAgentClient(address IPPort) *AgentClient {
	ac := new(AgentClient)
	ac.Host = address.IP
	ac.Port = address.Port
	return ac
}

// Get gets process info from kimo agent.
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

func findAgentProcess(addr IPPort, aps []*AgentProcess) *AgentProcess {
	for _, ap := range aps {
		if ap.Address.IP == addr.IP && ap.Address.Port == addr.Port {
			return ap
		}
	}
	return nil
}
