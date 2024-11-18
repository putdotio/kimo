package server

import (
	"context"
	"fmt"
	"kimo/config"
	"kimo/types"
	"strconv"
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
	Detail       *Detail

	AgentAddress types.IPPort
}

type Detail struct {
	Hostname string
	Status   Status
}

func (d *Detail) String() string {
	if d.Hostname != "" {
		return fmt.Sprintf("%s. Host: %s", d.Status.String(), d.Hostname)
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
	f.TCPProxyClient = NewTCPProxyClient(cfg)
	f.AgentPort = cfg.AgentPort

	return f
}

// GetAgentProcesseses gets processes from kimo agents
func (f *Fetcher) GetAgentProcesses(ctx context.Context, rps []*RawProcess) {
	log.Infof("Getting processes from %d agents...\n", len(rps))
	var wg sync.WaitGroup
	for _, rp := range rps {
		rps = append(rps, rp)

		// there is no connection on tcpproxy for this raw process.
		if rp.Detail != nil {
			if rp.Detail.Status == StatusProxyNotFound {
				continue
			}
		}

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
			rp.Detail = &Detail{Status: StatusAgentNotFound, Hostname: notFoundErr.Host}
		} else if cantConnectErr, ok := err.(*CantConnectError); ok {
			rp.Detail = &Detail{Status: StatusAgentCantConnect, Hostname: cantConnectErr.Host}
		} else {
			rp.Detail = &Detail{Status: StatusAgentError}
		}
	} else {
		rp.AgentProcess = ap
	}
}

func (f *Fetcher) combineMysqlAndProxyResults(rows []*MysqlRow, conns []*TCPProxyConn) []*RawProcess {
	log.Infoln("Combining mysql and tcpproxy results...")
	var rps []*RawProcess
	for _, row := range rows {
		rp := &RawProcess{MysqlRow: row}
		conn := findTCPProxyConn(row.Address, conns)
		if conn == nil {
			rp.AgentAddress = types.IPPort{IP: row.Address.IP, Port: row.Address.Port}
			rp.Detail = &Detail{Status: StatusProxyNotFound}
		} else {
			rp.AgentAddress = types.IPPort{IP: conn.ClientOut.IP, Port: conn.ClientOut.Port}
			rp.TCPProxyConn = conn
		}

		rps = append(rps, rp)
	}
	return rps
}

func (f *Fetcher) createKimoProcesses(rps []*RawProcess) []KimoProcess {
	kps := make([]KimoProcess, 0)
	for _, rp := range rps {
		ut, err := strconv.ParseUint(rp.MysqlRow.Time, 10, 32)
		if err != nil {
			log.Errorf("time %s could not be converted to int", rp.MysqlRow.Time)
		}
		kp := KimoProcess{
			ID:        rp.MysqlRow.ID,
			MysqlUser: rp.MysqlRow.User,
			DB:        rp.MysqlRow.DB.String,
			Command:   rp.MysqlRow.Command,
			Time:      uint32(ut),
			State:     rp.MysqlRow.State.String,
			Info:      rp.MysqlRow.Info.String,
		}
		if rp.AgentProcess != nil {
			kp.CmdLine = rp.AgentProcess.CmdLine
			kp.Pid = rp.AgentProcess.Pid
			kp.Host = rp.AgentProcess.Hostname
		} else {
			if rp.Detail != nil {
				if rp.Detail.Status == StatusAgentNotFound {
					kp.Host = rp.Detail.Hostname
				} else if rp.Detail.Status == StatusAgentCantConnect {
					kp.Host = rp.Detail.Hostname
				}
				kp.Detail = rp.Detail.String()
			}
		}

		kps = append(kps, kp)

	}
	return kps
}

// FetchAll is used to create processes from mysql to agents
func (f *Fetcher) FetchAll(ctx context.Context) ([]KimoProcess, error) {
	log.Debugf("Fetching...")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log.Infoln("Getting mysql results...")
	rows, err := f.MysqlClient.Get(ctx)
	if err != nil {
		return nil, err
	}
	log.Infof("Got %d mysql rows \n", len(rows))

	// TODO: check tcpproxy config first and then do this.
	log.Infoln("Getting tcpproxy results...")
	tps, err := f.TCPProxyClient.Get(ctx)
	if err != nil {
		return nil, err
	}
	log.Infof("Got %d tcpproxy conns \n", len(tps))

	// TODO: check tcpproxy config first and then do this
	rps := f.combineMysqlAndProxyResults(rows, tps)

	f.GetAgentProcesses(ctx, rps)
	ps := f.createKimoProcesses(rps)

	log.Debugf("%d processes are generated \n", len(ps))
	return ps, nil
}
