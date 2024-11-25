package agent

import (
	"context"
	"kimo/config"
	"net/http"
	"os"
	"sync"

	"github.com/cenkalti/log"
	gopsutilNet "github.com/shirou/gopsutil/v4/net"
)

// Agent is type for handling agent operations
type Agent struct {
	Config   *config.AgentConfig
	conns    []gopsutilNet.ConnectionStat
	Hostname string
	mu       sync.RWMutex // protects conns
}

// NewAgent creates an returns a new Agent
func NewAgent(cfg *config.AgentConfig) *Agent {
	d := new(Agent)
	d.Config = cfg
	d.Hostname = getHostname()
	return d
}

// SetConns sets connections with lock.
func (a *Agent) SetConns(conns []gopsutilNet.ConnectionStat) {
	a.mu.Lock()
	a.conns = conns
	a.mu.Unlock()
}

// GetConns gets connections with lock.
func (a *Agent) GetConns() []gopsutilNet.ConnectionStat {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.conns
}

// getHostname returns hostname.
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		log.Errorf("Hostname could not found")
		hostname = "UNKNOWN"
	}
	return hostname
}

// Run starts the http server and begins listening for HTTP requests.
func (a *Agent) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go a.pollConns(ctx)

	http.HandleFunc("/proc", a.Process)
	err := http.ListenAndServe(a.Config.ListenAddress, nil)
	if err != nil {
		log.Errorln(err.Error())
		return err
	}
	return nil
}
