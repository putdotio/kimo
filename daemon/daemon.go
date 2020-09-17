package daemon

import (
	"encoding/json"
	"fmt"
	"kimo/config"
	"kimo/types"
	"net/http"
	"os"
	"strconv"

	gopsutilNet "github.com/shirou/gopsutil/net"
	gopsutilProcess "github.com/shirou/gopsutil/process"
)

// Run on servers
// serve data through http api
// collect data when a new request comes through api
// use gopsutil

// Accept port as param
// return process info:
//  process name
//  host name
//

func NewDaemon(cfg *config.Daemon) *Daemon {
	d := new(Daemon)
	d.Config = cfg
	return d
}

type Daemon struct {
	Config *config.Daemon
}

func (d *Daemon) parsePorts(w http.ResponseWriter, req *http.Request) []uint32 {
	portsParam, ok := req.URL.Query()["ports"]
	fmt.Printf("ports: %s\n", portsParam)

	if !ok {
		fmt.Println("ports param is not provided.")
		return nil
	}

	var pl []uint32
	for _, port := range portsParam {
		p, err := strconv.ParseInt(port, 10, 32)
		if err != nil {
			fmt.Printf("error during string to int32: %s\n", err)
			continue
		}
		pl = append(pl, uint32(p))
	}
	if len(pl) < 1 {
		return nil
	}
	return pl
}

func (d *Daemon) isRequestedPort(localPort uint32, requestedPorts []uint32) bool {
	for _, requestedPort := range requestedPorts {
		if requestedPort == localPort {
			return true
		}
	}
	return false
}

func (d *Daemon) conns(w http.ResponseWriter, req *http.Request) {
	// todo: cache result for a short period (10s? 30s?)
	// todo: should server return real host ip & address if server is tcp proxy?
	ports := d.parsePorts(w, req)
	if ports == nil {
		http.Error(w, "ports param is required", http.StatusBadRequest)
		return
	}
	fmt.Println("ports: ", ports)

	connections, err := gopsutilNet.Connections("all")
	if err != nil {
		fmt.Println("Error while getting connections: ", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	processes, nil := gopsutilProcess.Processes()
	if err != nil {
		fmt.Println("Error while getting connections: ", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	daemonProcesses := make([]types.DaemonProcess, 0)

	for _, conn := range connections {
		if !d.isRequestedPort(conn.Laddr.Port, ports) {
			continue
		}

		process := d.findProcess(conn.Pid, processes)
		if err != nil {
			// todo: handle
			continue
		}

		name, err := process.Name()
		if err != nil {
			name = "" // todo: a const like "UNKNOWN"??
			// todo: handle
		}
		cmdline, err := process.Cmdline()
		if err != nil {
			cmdline = "" // todo: a const like "UNKNOWN"??
			// todo: handle
		}

		daemonProcesses = append(daemonProcesses, types.DaemonProcess{
			Laddr:   conn.Laddr,
			Status:  conn.Status,
			Pid:     conn.Pid,
			Name:    name,
			CmdLine: cmdline,
		})
	}
	w.Header().Set("Content-Type", "application/json")

	hostname, err := os.Hostname()
	if err != nil {
		// todo: handle error
	}

	response := &types.KimoDaemonResponse{
		Hostname:        hostname,
		DaemonProcesses: daemonProcesses,
	}
	json.NewEncoder(w).Encode(response)

}

func (d *Daemon) findProcess(pid int32, processes []*gopsutilProcess.Process) *gopsutilProcess.Process {
	for _, process := range processes {
		if process.Pid == pid {
			return process
		}
	}
	return nil

}

func (d *Daemon) Run() error {
	http.HandleFunc("/conns", d.conns)
	err := http.ListenAndServe(d.Config.ListenAddress, nil)
	if err != nil {
		return err
		// todo: handle error
	}
	return nil
}
