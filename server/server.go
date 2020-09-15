package server

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

// Run on server
// serve data through http api
// collect data when a new request comes through api
// use gopsutil

// Accept port as param
// return process info:
//  process name
//  host name
//

func NewServer(cfg *config.Server) *Server {
	s := new(Server)
	s.Config = cfg
	return s
}

type Server struct {
	Config *config.Server
}

func (s *Server) parsePorts(w http.ResponseWriter, req *http.Request) []uint32 {
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

func (s *Server) isRequestedPort(localPort uint32, requestedPorts []uint32) bool {
	for _, requestedPort := range requestedPorts {
		if requestedPort == localPort {
			return true
		}
	}
	return false
}

func (s *Server) conns(w http.ResponseWriter, req *http.Request) {
	ports := s.parsePorts(w, req)
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
	serverProcesses := make([]types.ServerProcess, 0)

	for _, conn := range connections {
		if !s.isRequestedPort(conn.Laddr.Port, ports) {
			continue
		}

		process := s.findProcess(conn.Pid, processes)
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

		serverProcesses = append(serverProcesses, types.ServerProcess{
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

	response := &types.KimoServerResponse{
		Hostname:        hostname,
		ServerProcesses: serverProcesses,
	}
	json.NewEncoder(w).Encode(response)

}

func (s *Server) findProcess(pid int32, processes []*gopsutilProcess.Process) *gopsutilProcess.Process {
	for _, process := range processes {
		if process.Pid == pid {
			return process
		}
	}
	return nil

}

func (s *Server) Run() error {
	http.HandleFunc("/conns", s.conns)
	err := http.ListenAndServe(s.Config.ListenAddress, nil)
	if err != nil {
		return err
		// todo: handle error
	}
	return nil
}
