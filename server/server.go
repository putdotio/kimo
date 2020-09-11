package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	gopsutilNet "github.com/shirou/gopsutil/net"
	gopsutilProcess "github.com/shirou/gopsutil/process"
)

// Run on server
// serve data through http api
// collect data when a new request comes through api
// use gopsutil

// Accept port as param
// return process info:
//  process name
//  host name
//

type Addr struct {
	IP   string `json:"ip"`
	Port uint32 `json:"port"`
}
type KimoProcess struct {
	Laddr  gopsutilNet.Addr `json:"localaddr"`
	Status string           `json:"status"`
	Pid    int32            `json:"pid"`
	// CmdLine string  `json:"cmdline"`  // how to get this?
	Name string `json:"name"`
}

type KimoResponse struct {
	Hostname      string        `json:"hostname"`
	HostProcesses []KimoProcess `json:"processes"`
}

func parsePorts(w http.ResponseWriter, req *http.Request) []uint32 {
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

func isRequestedPort(localPort uint32, requestedPorts []uint32) bool {
	for _, requestedPort := range requestedPorts {
		if requestedPort == localPort {
			return true
		}
	}
	return false
}

func conns(w http.ResponseWriter, req *http.Request) {
	ports := parsePorts(w, req)
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
	result := make([]KimoProcess, 0)

	for _, conn := range connections {
		if !isRequestedPort(conn.Laddr.Port, ports) {
			continue
		}

		process := findProcess(conn.Pid, processes)
		if err != nil {
			// todo: handle
			continue
		}

		name, err := process.Name()
		if err != nil {
			name = "" // todo: a const like "UNKNOWN"??
			// todo: handle
		}

		result = append(result, KimoProcess{
			Laddr:  conn.Laddr,
			Status: conn.Status,
			Pid:    conn.Pid,
			Name:   name,
		})
		// retrieve cmdline
	}
	w.Header().Set("Content-Type", "application/json")

	hostname, err := os.Hostname()
	if err != nil {
		// todo: handle error
	}

	response := &KimoResponse{
		Hostname:      hostname,
		HostProcesses: result,
	}
	json.NewEncoder(w).Encode(response)

}

func findProcess(pid int32, processes []*gopsutilProcess.Process) *gopsutilProcess.Process {
	for _, process := range processes {
		if process.Pid == pid {
			return process
		}
	}
	return nil

}

func Run() error {
	http.HandleFunc("/conns", conns)
	err := http.ListenAndServe("0.0.0.0:8090", nil)
	if err != nil {
		return err
		// todo: handle error
	}
	return nil
}
