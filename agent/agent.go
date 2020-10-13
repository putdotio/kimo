package agent

import (
	"encoding/json"
	"kimo/config"
	"kimo/types"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/cenkalti/log"
	gopsutilNet "github.com/shirou/gopsutil/net"
	gopsutilProcess "github.com/shirou/gopsutil/process"
)

// NewAgent is constuctor function for Agent type
func NewAgent(cfg *config.Config) *Agent {
	d := new(Agent)
	d.Config = &cfg.Agent
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

// Agent is type for handling agent operations
type Agent struct {
	Config   *config.Agent
	Conns    []gopsutilNet.ConnectionStat
	Hostname string
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

// Process is handler for serving process info
func (a *Agent) Process(w http.ResponseWriter, req *http.Request) {
	// todo: cache result for a short period (10s? 30s?)
	port, err := parsePortParam(w, req)
	if err != nil {
		http.Error(w, "port param is required", http.StatusBadRequest)
		return
	}
	for _, conn := range a.Conns {
		if conn.Laddr.Port != port {
			continue
		}

		if conn.Pid == 0 {
			continue
		}

		process, err := gopsutilProcess.NewProcess(conn.Pid)
		if err != nil {
			log.Debugf("Error occured while finding the process %s\n", err.Error())
			continue
		}
		if process == nil {
			log.Debugf("Process could not found for %d\n", conn.Pid)
			continue
		}

		name, err := process.Name()
		if err != nil {
			name = ""
		}
		cls, err := process.CmdlineSlice()
		if err != nil {
			log.Debugf("Cmdline could not found for %d\n", process.Pid)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(types.AgentProcess{
			Laddr:    types.IPPort{IP: conn.Laddr.IP, Port: conn.Laddr.Port},
			Status:   conn.Status,
			Pid:      conn.Pid,
			Name:     name,
			CmdLine:  cls,
			Hostname: a.Hostname,
		})
		return
	}
	http.Error(w, "process not found!", http.StatusNotFound)
	return
}

func (a *Agent) pollConns() {
	// todo: run with context
	log.Debugln("Polling...")
	ticker := time.NewTicker(a.Config.PollDuration * time.Second)

	for {
		a.getConns() // poll immediately at the initialization
		select {
		// todo: add return case
		case <-ticker.C:
			a.getConns()
		}
	}

}
func (a *Agent) getConns() {
	// This is an expensive operation.
	// So, we need to call infrequent to prevent high load on servers those run kimo agents.
	conns, err := gopsutilNet.Connections("all")
	if err != nil {
		log.Errorln(err.Error())
		return
	}
	a.Conns = conns
}

// Run is main function to run http server
func (a *Agent) Run() error {
	go a.pollConns()

	http.HandleFunc("/proc", a.Process)
	err := http.ListenAndServe(a.Config.ListenAddress, nil)
	if err != nil {
		log.Errorln(err.Error())
		return err
	}
	return nil
}
