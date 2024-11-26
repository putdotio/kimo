package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cenkalti/log"
	gopsutilNet "github.com/shirou/gopsutil/v4/net"
	gopsutilProcess "github.com/shirou/gopsutil/v4/process"
)

// Process contains basic process information for API responses.
type Process struct {
	Status  string `json:"status"`
	Pid     int32  `json:"pid"`
	Port    int32  `json:"port"`
	Name    string `json:"name"`
	CmdLine string `json:"cmdline"`
}

// Response contains basic process information for API responses.
type Response struct {
	Processes []*Process `json:"processes"`
}

// parsePortsParam parses and returns port numbers from the request.
func parsePortsParam(w http.ResponseWriter, req *http.Request) ([]uint32, error) {
	portsParam := req.URL.Query().Get("ports")
	log.Debugf("Looking for process(es) for ports: %s\n", portsParam)

	if portsParam == "" {
		log.Errorln("ports param is not provided.")
		return nil, fmt.Errorf("ports param is required") //todo: check again.
	}

	ports := strings.Split(portsParam, ",")

	var portNumbers []uint32
	for _, port := range ports {
		p, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("invalid port number: %s", port)
		}
		portNumbers = append(portNumbers, uint32(p))
	}

	return portNumbers, nil
}

// findProcesses finds process(es) those have connections with given ports.
func findProcesses(ports []uint32, conns []gopsutilNet.ConnectionStat) []*Process {
	ps := make([]*Process, 0)

	for _, conn := range conns {
		if !portExists(conn.Laddr.Port, ports) {
			continue
		}

		process, err := gopsutilProcess.NewProcess(conn.Pid)
		if err != nil {
			log.Debugf("Error occured while finding the process %s\n", err.Error())
			continue
		}

		if process == nil {
			log.Debugf("Process not found for %d\n", conn.Pid)
			continue
		}

		name, err := process.Name()
		if err != nil {
			log.Debugf("Name not found for %d\n", process.Pid)
		}

		cmdline, err := process.Cmdline()
		if err != nil {
			log.Debugf("Cmdline not found for %d\n", process.Pid)
		}

		p := &Process{
			Status:  conn.Status,
			Pid:     conn.Pid,
			Port:    int32(conn.Laddr.Port),
			Name:    name,
			CmdLine: cmdline,
		}

		ps = append(ps, p)
	}
	return ps
}

func portExists(port uint32, ports []uint32) bool {
	for _, p := range ports {
		if p == port {
			return true
		}
	}
	return false
}

// Process is handler for serving process info
func (a *Agent) Process(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("X-Kimo-Hostname", a.Hostname)

	// todo: cache result for a short period (10s? 30s?)
	ports, err := parsePortsParam(w, req)
	if err != nil {
		http.Error(w, "port param is required", http.StatusBadRequest)
		return
	}
	ps := findProcesses(ports, a.GetConns())
	if ps == nil {
		http.Error(w, "Connection(s) not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := &Response{
		Processes: ps,
	}
	err = json.NewEncoder(w).Encode(&response)
	if err != nil {
		http.Error(w, "Can not encode agent process", http.StatusInternalServerError)
	}
}
