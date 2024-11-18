package server

import (
	"context"
	"fmt"
	"kimo/config"
	"kimo/types"
	"sync"

	"github.com/cenkalti/log"
)

// Fetcher fetches process info from mysql to kimo agent
type Fetcher struct {
	MysqlClient    *MysqlClient
	TCPProxyClient *TCPProxyClient

	AgentPort uint32
}

// RawProcess combines mysql row, tcp proxy conn and agent process
type RawProcess struct {
	MysqlRow     *MysqlRow
	TCPProxyConn *TCPProxyConn
	AgentProcess *types.AgentProcess
	Details      []*Detail

	AgentAddress types.IPPort
}

func (rp *RawProcess) Detail() string {
	var s string
	if rp.Details == nil {
		return ""
	}

	for _, d := range rp.Details {
		s += d.String() + " "
	}
	return s
}

type Detail struct {
	Hostname string
	Status   Status
}

func (d *Detail) String() string {
	if d.Hostname != "" {
		return fmt.Sprintf("%s - Host: %s.", d.Status.String(), d.Hostname)
	} else {
		return fmt.Sprintf("%s.", d.Status.String())
	}
}

type Status int

const (
	StatusAgentNotFound Status = iota
	StatusAgentCantConnect
	StatusAgentError
	StatusProxyNotFound
)

func (s Status) String() string {
	switch s {
	case StatusAgentError:
		return "Agent returned error"
	case StatusAgentNotFound:
		return "Process not found on agent"
	case StatusAgentCantConnect:
		return "Cant connect to agent"
	case StatusProxyNotFound:
		return "Connection not found on proxy"
	default:
		return "UNKNOWN"
	}
}

// NewFetcher is constructor fuction for creating a new Fetcher object
func NewFetcher(cfg config.Server) *Fetcher {
	f := new(Fetcher)
	f.MysqlClient = NewMysqlClient(cfg)
	if cfg.TCPProxyMgmtAddress != "" {
		f.TCPProxyClient = NewTCPProxyClient(cfg)
	}
	f.AgentPort = cfg.AgentPort
	return f
}

// GetAgentProcesseses gets processes from kimo agents
func (f *Fetcher) GetAgentProcesses(ctx context.Context, rps []*RawProcess) {
	log.Infof("Getting processes from %d agents...\n", len(rps))
	var wg sync.WaitGroup
	for _, rp := range rps {
		rps = append(rps, rp)

		wg.Add(1)
		go f.getAgentProcess(ctx, &wg, rp)
	}
	wg.Wait()
	log.Infoln("All agents are visited.")
}

func (f *Fetcher) getAgentProcess(ctx context.Context, wg *sync.WaitGroup, rp *RawProcess) {
	defer wg.Done()

	ac := NewAgentClient(rp.AgentAddress.IP, f.AgentPort)
	ap, err := ac.Get(ctx, rp.AgentAddress.Port)
	if err != nil {
		if notFoundErr, ok := err.(*NotFoundError); ok {
			rp.Details = append(rp.Details, &Detail{Status: StatusAgentNotFound, Hostname: notFoundErr.Host})
		} else if _, ok := err.(*CantConnectError); ok {
			h := fmt.Sprintf("%s:%d", rp.AgentAddress.IP, f.AgentPort)
			rp.Details = append(rp.Details, &Detail{Status: StatusAgentCantConnect, Hostname: h})
		} else {
			rp.Details = append(rp.Details, &Detail{Status: StatusAgentError})
		}
	} else {
		rp.AgentProcess = ap
	}
}

func addProxyConns(rps []*RawProcess, conns []*TCPProxyConn) {
	log.Infoln("Adding tcpproxy conns...")
	for _, rp := range rps {
		conn := findTCPProxyConn(rp.AgentAddress, conns)
		if conn != nil {
			rp.AgentAddress = types.IPPort{IP: conn.ClientOut.IP, Port: conn.ClientOut.Port}
			rp.TCPProxyConn = conn
		}
	}
}

func createRawProcesses(rows []*MysqlRow) []*RawProcess {
	log.Infoln("Combining mysql and tcpproxy results...")
	var rps []*RawProcess
	for _, row := range rows {
		rp := &RawProcess{MysqlRow: row}
		rp.AgentAddress = types.IPPort{IP: row.Address.IP, Port: row.Address.Port}
		rps = append(rps, rp)
	}
	return rps
}

// FetchAll is used to create processes from mysql to agents
func (f *Fetcher) FetchAll(ctx context.Context) ([]*RawProcess, error) {
	log.Debugf("Fetching...")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log.Infoln("Getting mysql results...")
	rows, err := f.MysqlClient.Get(ctx)
	if err != nil {
		return nil, err
	}
	log.Infof("Got %d mysql rows \n", len(rows))

	rps := createRawProcesses(rows)

	if f.TCPProxyClient != nil {
		log.Infoln("Getting tcpproxy conns...")
		tps, err := f.TCPProxyClient.Get(ctx)
		if err != nil {
			return nil, err
		}
		log.Infof("Got %d tcpproxy conns \n", len(tps))
		addProxyConns(rps, tps)
	}

	f.GetAgentProcesses(ctx, rps)

	log.Debugf("%d raw processes are generated \n", len(rps))
	return rps, nil
}
