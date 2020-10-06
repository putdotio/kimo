package daemon

import (
	"encoding/json"
	"kimo/config"
	"kimo/types"
	"net/http"
	"os"
	"strconv"

	"github.com/cenkalti/log"
	gopsutilNet "github.com/shirou/gopsutil/net"
	gopsutilProcess "github.com/shirou/gopsutil/process"
)

func NewDaemon(cfg *config.Config) *Daemon {
	d := new(Daemon)
	d.Config = &cfg.Daemon
	return d
}

type Daemon struct {
	Config *config.Daemon
}

func (d *Daemon) parsePortParam(w http.ResponseWriter, req *http.Request) (uint32, error) {
	portParam, ok := req.URL.Query()["port"]
	log.Infof("Looking for process of port: %s\n", portParam)

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

func (d *Daemon) Process(w http.ResponseWriter, req *http.Request) {
	// todo: cache result for a short period (10s? 30s?)
	port, err := d.parsePortParam(w, req)
	if err != nil {
		http.Error(w, "port param is required", http.StatusBadRequest)
		return
	}
	connections, err := gopsutilNet.Connections("all")
	if err != nil {
		log.Errorf("Error while getting connections: %s\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Errorf("Hostname could not found")
		hostname = "UNKNOWN"
	}

	for _, conn := range connections {
		if conn.Laddr.Port != port {
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
		json.NewEncoder(w).Encode(types.DaemonProcess{
			Laddr:    types.IPPort{IP: conn.Laddr.IP, Port: conn.Laddr.Port},
			Status:   conn.Status,
			Pid:      conn.Pid,
			Name:     name,
			CmdLine:  cls,
			Hostname: hostname,
		})
		return
	}
	http.Error(w, "process not found!", http.StatusNotFound)
	return
}

func (d *Daemon) Run() error {
	http.HandleFunc("/proc", d.Process)
	err := http.ListenAndServe(d.Config.ListenAddress, nil)
	if err != nil {
		log.Errorln(err.Error())
		return err
	}
	return nil
}
