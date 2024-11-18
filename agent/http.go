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
	gopsutilNet "github.com/shirou/gopsutil/v4/net"
	gopsutilProcess "github.com/shirou/gopsutil/v4/process"
)

// Agent is type for handling agent operations
type Agent struct {
	Config   *config.Agent
	Conns    []gopsutilNet.ConnectionStat
	Hostname string
}

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

type hostProc struct {
	process *gopsutilProcess.Process
	conn    gopsutilNet.ConnectionStat
}

func (a *Agent) findProc(port uint32) *hostProc {
	for _, conn := range a.Conns {
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

		return &hostProc{
			process: process,
			conn:    conn,
		}
	}
	return nil
}

func (a *Agent) createAgentProcess(proc *hostProc) *types.AgentProcess {
	if proc == nil {
		return nil
	}
	name, err := proc.process.Name()
	if err != nil {
		name = ""
	}
	cl, err := proc.process.CmdlineSlice()
	if err != nil {
		log.Debugf("Cmdline could not found for %d\n", proc.process.Pid)
	}
	return &types.AgentProcess{
		Laddr:    types.IPPort{IP: proc.conn.Laddr.IP, Port: proc.conn.Laddr.Port},
		Status:   proc.conn.Status,
		Pid:      proc.conn.Pid,
		Name:     name,
		CmdLine:  cl,
		Hostname: a.Hostname,
	}
}

// Process is handler for serving process info
func (a *Agent) Process(w http.ResponseWriter, req *http.Request) {
	// todo: cache result for a short period (10s? 30s?)
	port, err := parsePortParam(w, req)
	if err != nil {
		http.Error(w, "port param is required", http.StatusBadRequest)
		return
	}
	p := a.findProc(port)
	ap := a.createAgentProcess(p)

	w.Header().Set("Content-Type", "application/json")
	if ap == nil {
		w.Header().Set("X-Hostname", a.Hostname)
		http.Error(w, "Can not create agent process", http.StatusNotFound)
		return
	}
	err = json.NewEncoder(w).Encode(&ap)
	if err != nil {
		http.Error(w, "Can not encode agent process", http.StatusInternalServerError)
	}
}

func (a *Agent) pollConns() {
	// todo: run with context
	log.Debugln("Polling...")
	ticker := time.NewTicker(a.Config.PollInterval * time.Second)

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
	// So, we need to call it infrequent to prevent high load on servers those run kimo agents.
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
