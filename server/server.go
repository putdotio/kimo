package server

import (
	"context"
	"encoding/json"
	"fmt"
	"kimo/config"
	"kimo/types"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/cenkalti/log"
	"github.com/rakyll/statik/fs"

	_ "kimo/statik" // Auto-generated module by statik.
)

// Response is type for returning a response from kimo server
type Response struct {
	Processes []Process `json:"processes"`
}

// Process is the final processes that is combined with DaemonProcess + TCPProxyRecord + MysqlProcess
type Process struct {
	ID        int32    `json:"id"`
	MysqlUser string   `json:"mysql_user"`
	DB        string   `json:"db"`
	Command   string   `json:"command"`
	Time      uint32   `json:"time"`
	State     string   `json:"state"`
	Info      string   `json:"info"`
	CmdLine   []string `json:"cmdline"`
	Pid       int32    `json:"pid"`
	Host      string   `json:"host"`
}

// InitializeKimoProcesses initializes kimo processes combined with tcpproxy records and mysql processes
func (s *Server) InitializeKimoProcesses(mps []*MysqlProcess, tprs []*TCPProxyRecord) error {
	log.Infoln("Initializing Kimo processes...")
	for _, mp := range mps {
		tpr := findTCPProxyRecord(mp.Address, tprs)
		if tpr == nil {
			continue
		}
		s.KimoProcesses = append(s.KimoProcesses, &KimoProcess{
			MysqlProcess:   mp,
			TCPProxyRecord: tpr,
			Server:         s,
		})
	}
	log.Infof("%d processes are initialized \n", len(s.KimoProcesses))
	return nil
}

func findHostIP(host string) (string, error) {
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil {
			return "", err
		}
		return string(ips[0].String()), nil
	}
	return ip.String(), nil
}

func findTCPProxyRecord(addr types.IPPort, proxyRecords []*TCPProxyRecord) *TCPProxyRecord {
	ipAddr, err := findHostIP(addr.IP)
	if err != nil {
		log.Debugln(err.Error())
		return nil
	}

	for _, pr := range proxyRecords {
		if pr.ProxyOut.IP == ipAddr && pr.ProxyOut.Port == addr.Port {
			return pr
		}
	}
	return nil
}

// Init is used for setting up kimo processes
func (s *Server) Init(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errC := make(chan error)

	mysqlProcsC := make(chan []*MysqlProcess)
	proxyRecordsC := make(chan []*TCPProxyRecord)

	var mysqlProcesses []*MysqlProcess
	var tcpProxyRecords []*TCPProxyRecord

	go s.Mysql.FetchProcesses(ctx, mysqlProcsC, errC)
	go s.TCPProxy.FetchRecords(ctx, proxyRecordsC, errC)
	for {
		if mysqlProcesses != nil && tcpProxyRecords != nil {
			return s.InitializeKimoProcesses(mysqlProcesses, tcpProxyRecords)
		}
		select {
		case mps := <-mysqlProcsC:
			mysqlProcesses = mps
		case tprs := <-proxyRecordsC:
			tcpProxyRecords = tprs
		case err := <-errC:
			log.Errorf("Error occured: %s", err.Error())
			cancel()
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

}

// GenerateKimoProcesses is used to combine all information (mysql + tcpproxy + daemon) of a process
func (s *Server) GenerateKimoProcesses(ctx context.Context) {
	log.Infof("Generating %d kimo processes...\n", len(s.KimoProcesses))
	var wg sync.WaitGroup
	for _, kp := range s.KimoProcesses {
		wg.Add(1)
		go kp.SetDaemonProcess(ctx, &wg)
	}
	wg.Wait()
	log.Infoln("Generating process is done...")

}

// ReturnResponse is used to return a response from server
func (s *Server) ReturnResponse(ctx context.Context, w http.ResponseWriter) {
	log.Infof("Returning response with %d kimo processes...\n", len(s.KimoProcesses))
	processes := make([]Process, 0)
	for _, kp := range s.KimoProcesses {

		ut, err := strconv.ParseUint(kp.MysqlProcess.Time, 10, 32)
		if err != nil {
			log.Errorf("time %s could not be converted to int", kp.MysqlProcess.Time)
		}
		t := uint32(ut)

		processes = append(processes, Process{
			ID:        kp.MysqlProcess.ID,
			MysqlUser: kp.MysqlProcess.User,
			DB:        kp.MysqlProcess.DB.String,
			Command:   kp.MysqlProcess.Command,
			Time:      t,
			State:     kp.MysqlProcess.State.String,
			Info:      kp.MysqlProcess.Info.String,
			CmdLine:   kp.DaemonProcess.CmdLine,
			Pid:       kp.DaemonProcess.Pid,
			Host:      kp.DaemonProcess.Hostname,
		})
	}

	w.Header().Set("Content-Type", "application/json")

	response := &Response{
		Processes: processes,
	}
	json.NewEncoder(w).Encode(response)
}

// NewServer is used to create a new Server type
func NewServer(cfg *config.Config) *Server {
	log.Infoln("Creating a new server...")
	s := new(Server)
	s.Config = &cfg.Server
	s.Mysql = NewMysql(s.Config.DSN)
	s.TCPProxy = NewTCPProxy(s.Config.TCPProxyMgmtAddress, s.Config.TCPProxyConnectTimeout, s.Config.TCPProxyReadTimeout)
	return s
}

// Server is a type for handling server side
type Server struct {
	Config        *config.Server
	Mysql         *Mysql
	Server        *Server
	TCPProxy      *TCPProxy
	KimoProcesses []*KimoProcess
}

// Processes is a handler for returning process list
func (s *Server) Processes(w http.ResponseWriter, req *http.Request) {
	forceParam := req.URL.Query().Get("force")
	poll := false
	if forceParam == "true" || len(s.KimoProcesses) == 0 {
		poll = true
	}

	if poll {
		s.Poll()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "access-control-allow-origin, access-control-allow-headers")

	s.ReturnResponse(ctx, w)

}

// Health is a dummy endpoint for load balancer health check
func (s *Server) Health(w http.ResponseWriter, req *http.Request) {
	// todo: real health check
	fmt.Fprintf(w, "OK")
}

func (s *Server) Poll() {
	log.Debugf("Polling...")
	// todo: configurable time
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.KimoProcesses = make([]*KimoProcess, 0)
	err := s.Init(ctx)
	if err != nil {
		log.Errorln(err.Error())
		return
	}
	s.GenerateKimoProcesses(ctx)
}

// FetchProcesses polls processes periodically
func (s *Server) FetchProcesses() {
	ticker := time.NewTicker(s.Config.PollDuration * time.Second)

	for {
		s.Poll() // poll immediately at initialization
		select {
		// todo: add return case
		case <-ticker.C:
			s.Poll()
		}
	}

}

// Static serves static files (web components).
func (s *Server) Static() http.Handler {
	statikFS, err := fs.New()
	if err != nil {
		log.Errorln(err)
	}
	return http.FileServer(statikFS)

}

// todo: reconsider context usages
// Run is used to run http handlers
func (s *Server) Run() error {
	log.Infof("Running server on %s \n", s.Config.ListenAddress)

	go s.FetchProcesses()

	http.Handle("/", s.Static())
	http.HandleFunc("/procs", s.Processes)
	http.HandleFunc("/health", s.Health)
	err := http.ListenAndServe(s.Config.ListenAddress, nil)
	if err != nil {
		log.Errorln(err.Error())
		return err
	}
	return nil
}
