package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cenkalti/log"
)

// AgentProcess represents process info from a kimo-agent enhanced with response detail.
type AgentProcess struct {
	Pid              uint32
	Port             uint32 // process uses this port to communicate with MySQL.
	Name             string
	Cmdline          string
	ConnectionStatus string
	IP               string // kimo agent's IP.
	hostname         string
	err              error
}

// NewAgentProcess creates and returns a new AgentProcess.
func NewAgentProcesss(ar *AgentResponse, ip string, port uint32, err error) *AgentProcess {
	ap := new(AgentProcess)
	ap.err = err
	ap.IP = ip
	ap.Port = port

	if ar != nil {
		ap.Pid = ar.Pid
		ap.Name = ar.Name
		ap.Cmdline = ar.CmdLine
		ap.hostname = ar.Hostname
		ap.ConnectionStatus = ar.Status
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
	return ap.IP
}

// AgentClient represents an agent client to fetch get process from a kimo-agent
type AgentClient struct {
	Address IPPort
}

// AgentError represents an HTTP error that is retured from kimo-agent.
type AgentError struct {
	Hostname string
	Status   string
}

// AgentResponse represents a success response from kimo-agent.
type AgentResponse struct {
	Hostname string
	Status   string
	Pid      uint32
	Name     string
	CmdLine  string
	Port     uint32
}

func (ae *AgentError) Error() string {
	return fmt.Sprintf("Agent error. Host: %s - status: %s\n", ae.Hostname, ae.Status)
}

// NewAgentClient creates and returns a new AgentClient.
func NewAgentClient(address IPPort) *AgentClient {
	// kimo-agent listens this address
	return &AgentClient{Address: address}
}

// Get gets process info from kimo agent.
func (ac *AgentClient) Get(ctx context.Context, ports []uint32) ([]*AgentResponse, error) {
	url := fmt.Sprintf("http://%s:%d/proc?ports=%s", ac.Address.IP, ac.Address.Port, createPortsParam(ports))
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

	type process struct {
		Status  string `json:"status"`
		Pid     uint32 `json:"pid"`
		Port    uint32 `json:"port"`
		Name    string `json:"name"`
		CmdLine string `json:"cmdline"`
	}

	type Response struct {
		Processes []*process `json:"processes"`
	}

	var r Response
	err = json.NewDecoder(response.Body).Decode(&r)

	if err != nil {
		log.Errorln(err.Error())
		return nil, err
	}

	ars := make([]*AgentResponse, 0)
	for _, p := range r.Processes {
		ar := &AgentResponse{
			Status:   p.Status,
			Pid:      p.Pid,
			Port:     p.Port,
			Name:     p.Name,
			CmdLine:  p.CmdLine,
			Hostname: hostname,
		}
		ars = append(ars, ar)
	}

	return ars, nil

}
func createPortsParam(ports []uint32) string {
	numbers := make([]string, len(ports))
	for i, port := range ports {
		numbers[i] = fmt.Sprint(port)
	}
	return strings.Join(numbers, ",")
}

func findAgentProcess(addr IPPort, aps []*AgentProcess) *AgentProcess {
	for _, ap := range aps {
		if ap.IP == addr.IP && ap.Port == addr.Port {
			return ap
		}
	}
	return nil
}
