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
	conns    []Conn
	Hostname string
	mu       sync.RWMutex // protects conns
}

type Conn struct {
	Port   uint32
	Pid    int32
	Status string
}

// NewAgent creates an returns a new Agent
func NewAgent(cfg *config.AgentConfig) *Agent {
	d := new(Agent)
	d.Config = cfg
	d.Hostname = getHostname()
	return d
}

// SetConns sets connections with lock.
func (a *Agent) SetConns(conns []Conn) {
	a.mu.Lock()
	a.conns = conns
	a.mu.Unlock()
}

// GetConns gets connections with lock.
func (a *Agent) GetConns() []Conn {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.conns
}

func (a *Agent) ConvertConns(gopsConns []gopsutilNet.ConnectionStat) []Conn {
	conns := make([]Conn, 0)
	for _, cs := range gopsConns {
		conn := Conn{
			Port:   cs.Laddr.Port,
			Status: cs.Status,
			Pid:    cs.Pid,
		}
		conns = append(conns, conn)
	}
	return conns
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
	log.Infof("Running server on %s \n", a.Config.ListenAddress)
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
