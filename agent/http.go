package agent

import (
	"context"
	"encoding/json"
	"kimo/config"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/cenkalti/log"
	gopsutilNet "github.com/shirou/gopsutil/v4/net"
	gopsutilProcess "github.com/shirou/gopsutil/v4/process"
)

// Agent is type for handling agent operations
type Agent struct {
	Config   *config.AgentConfig
	conns    []gopsutilNet.ConnectionStat
	Hostname string
	mu       sync.RWMutex // protects conns
}

func (a *Agent) SetConns(conns []gopsutilNet.ConnectionStat) {
	a.mu.Lock()
	a.conns = conns
	a.mu.Unlock()
}

func (a *Agent) GetConns() []gopsutilNet.ConnectionStat {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.conns
}

type Response struct {
	Status  string `json:"status"`
	Pid     int32  `json:"pid"`
	Name    string `json:"name"`
	CmdLine string `json:"cmdline"`
}

// NewAgent is constuctor function for Agent type
func NewAgent(cfg *config.AgentConfig) *Agent {
	d := new(Agent)
	d.Config = cfg
	d.Hostname = getHostname()
	return d
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		log.Errorf("Hostname could not found")
		hostname = "UNKNOWN"
	}
	return hostname
}

func parsePortParam(w http.ResponseWriter, req *http.Request) (uint32, error) {
	portParam, ok := req.URL.Query()["port"]
	log.Debugf("Looking for process of port: %s\n", portParam)

	if !ok || len(portParam) < 1 {
		log.Errorln("port param is not provided.")
		return 0, nil
	}

	p, err := strconv.ParseInt(portParam[0], 10, 32)
	if err != nil {
		log.Errorln("error during string to int32: %s\n", err)
		return 0, err
	}
	return uint32(p), nil
}

type NetworkProcess struct {
	process *gopsutilProcess.Process
	conn    gopsutilNet.ConnectionStat
}

func (a *Agent) findProcess(port uint32) *NetworkProcess {
	for _, conn := range a.GetConns() {
		if conn.Laddr.Port != port {
			continue
		}

		process, err := gopsutilProcess.NewProcess(conn.Pid)
		if err != nil {
			log.Debugf("Error occured while finding the process %s\n", err.Error())
			return nil
		}

		if process == nil {
			log.Debugf("Process could not found for %d\n", conn.Pid)
			return nil
		}

		return &NetworkProcess{
			process: process,
			conn:    conn,
		}
	}
	return nil
}

func createResponse(np *NetworkProcess) *Response {
	if np == nil {
		return nil
	}
	name, err := np.process.Name()
	if err != nil {
		name = ""
	}
	cmdline, err := np.process.Cmdline()
	if err != nil {
		log.Debugf("Cmdline could not found for %d\n", np.process.Pid)
	}
	return &Response{
		Status:  np.conn.Status,
		Pid:     np.conn.Pid,
		Name:    name,
		CmdLine: cmdline,
	}
}

// Process is handler for serving process info
func (a *Agent) Process(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("X-Kimo-Hostname", a.Hostname)

	// todo: cache result for a short period (10s? 30s?)
	port, err := parsePortParam(w, req)
	if err != nil {
		http.Error(w, "port param is required", http.StatusBadRequest)
		return
	}
	p := a.findProcess(port)
	if p == nil {
		http.Error(w, "Connection not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ap := createResponse(p)
	err = json.NewEncoder(w).Encode(&ap)
	if err != nil {
		http.Error(w, "Can not encode agent process", http.StatusInternalServerError)
	}
}

// Run is main function to run http server
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
