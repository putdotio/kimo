package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cenkalti/log"
)

// AgentProcess represents process info from a kimo-agent
type AgentProcess struct {
	ConnectionStatus string `json:"status"`
	Pid              uint32 `json:"pid"`
	Port             uint32 `json:"port"` // process uses this port to communicate with MySQL.
	Name             string `json:"name"`
	Cmdline          string `json:"cmdline"`
}

// EnhancedAgentProcess represents process info along with agent's and connection's properties (error, hostname etc.)
type EnhancedAgentProcess struct {
	AgentProcess
	hostname string
	ip       string
	err      error
}

// Host is kimo agent's hostname if response is returned, otherwise host's IP.
func (eap *EnhancedAgentProcess) Host() string {
	if eap.hostname != "" {
		return eap.hostname
	}
	return eap.ip
}

// AgentResponse combines kimo-agent's response and http response detail.
type AgentResponse struct {
	err      error
	hostname string
	ip       string

	Processes []*AgentProcess
}

// AgentClient represents an agent client to fetch get process from a kimo-agent
type AgentClient struct {
	Address IPPort // kimo-agent listens this address
}

// NewAgentClient creates and returns a new AgentClient.
func NewAgentClient(address IPPort) *AgentClient {
	return &AgentClient{Address: address}
}

// Get gets process info from kimo agent.
func (ac *AgentClient) Get(ctx context.Context, ports []uint32) *AgentResponse {
	url := fmt.Sprintf("http://%s:%d/proc?ports=%s", ac.Address.IP, ac.Address.Port, createPortsParam(ports))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	if err != nil {
		return &AgentResponse{ip: ac.Address.IP, err: err}
	}
	client := &http.Client{}
	log.Debugf("Requesting to %s\n", url)
	response, err := client.Do(req)
	if err != nil {
		return &AgentResponse{ip: ac.Address.IP, err: err}
	}

	defer response.Body.Close()
	hostname := response.Header.Get("X-Kimo-Hostname")
	if response.StatusCode != 200 {
		return &AgentResponse{ip: ac.Address.IP, err: fmt.Errorf(response.Status), hostname: hostname}
	}

	var r struct {
		Processes []*AgentProcess `json:"processes"`
	}
	err = json.NewDecoder(response.Body).Decode(&r)

	if err != nil {
		log.Errorln(err.Error())
		return &AgentResponse{ip: ac.Address.IP, err: err, hostname: hostname}
	}

	return &AgentResponse{ip: ac.Address.IP, hostname: hostname, Processes: r.Processes}

}

// createPortsParam creates comma seperated ports param from given slice of port numbers.
func createPortsParam(ports []uint32) string {
	numbers := make([]string, len(ports))
	for i, port := range ports {
		numbers[i] = fmt.Sprint(port)
	}
	return strings.Join(numbers, ",")
}

// findProcess finds EnhancedAgentProcess for given port from agent responses.
func findProcess(addr IPPort, ars []*AgentResponse) *EnhancedAgentProcess {
	for _, ar := range ars {
		if addr.IP == ar.ip {
			eap := &EnhancedAgentProcess{ // kimo-agent returns response
				hostname: ar.hostname,
				ip:       ar.ip,
				err:      ar.err,
			}
			for _, ap := range ar.Processes {
				if addr.Port == ap.Port { // kimo-agent returns response with process
					eap.AgentProcess = *ap
					break
				}
			}
			return eap
		}
	}
	return nil
}
