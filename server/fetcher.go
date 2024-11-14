package server

import (
	"context"
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

	Detail Detail
}

type Detail int

const (
	DetailAgentFound Detail = iota
	DetailAgentNotFound
	DetailAgentCantConnect
	DetailAgentError
	DetailProxyNotFound
)

func (d Detail) String() string {
	switch d {
	case DetailAgentFound:
		return "Found on agent"
	case DetailAgentError:
		return "Agent errored"
	case DetailAgentNotFound:
		return "Not found on agent"
	case DetailAgentCantConnect:
		return "Cant connect to agent"
	case DetailProxyNotFound:
		return "Not found on proxy"
	default:
		return "UNKNOWN"
	}
}
func (rp *RawProcess) Addr() types.IPPort {
	var ip string
	var port uint32
	if rp.TCPProxyConn != nil {
		ip = rp.TCPProxyConn.ClientOut.IP
		port = rp.TCPProxyConn.ClientOut.Port
	} else {
		ip = rp.MysqlRow.Address.IP
		port = rp.MysqlRow.Address.Port
	}
	return types.IPPort{IP: ip, Port: port}

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

		wg.Add(1)
		go f.getAgentProcess(ctx, &wg, rp)
	}
	wg.Wait()
	log.Infoln("All agents are visited.")
}

func (f *Fetcher) getAgentProcess(ctx context.Context, wg *sync.WaitGroup, rp *RawProcess) {
	defer wg.Done()

	ac := NewAgentClient(rp.Addr().IP, f.AgentPort)
	ap, err := ac.Get(ctx, rp.Addr().Port)
	if err != nil {
		if notFoundErr, ok := err.(*NotFoundError); ok {
			rp.Detail = DetailAgentNotFound
			rp.AgentProcess = &types.AgentProcess{
				Hostname: notFoundErr.Host,
			}
		} else if cantConnectErr, ok := err.(*CantConnectError); ok {
			rp.Detail = DetailAgentCantConnect
			rp.AgentProcess = &types.AgentProcess{
				Hostname: cantConnectErr.Host,
			}
		} else {
			rp.Detail = DetailAgentError
		}
	} else {
		rp.AgentProcess = ap
		rp.Detail = DetailAgentFound
	}
}

func (f *Fetcher) createRawProcesses(rows []*MysqlRow, conns []*TCPProxyConn) []*RawProcess {
	log.Infoln("Creating raw processes...")
	var rps []*RawProcess
	for _, row := range rows {
		rp := &RawProcess{MysqlRow: row}
		conn := findTCPProxyConn(row.Address, conns)
		if conn == nil {
			rp.Detail = DetailProxyNotFound
		} else {
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
			Detail:    rp.Detail.String(),
		}
		if rp.AgentProcess != nil {
			if rp.Detail == DetailAgentFound {
				kp.CmdLine = rp.AgentProcess.CmdLine
				kp.Pid = rp.AgentProcess.Pid
				kp.Host = rp.AgentProcess.Hostname
			}

			if rp.Detail == DetailAgentNotFound {
				kp.Host = rp.AgentProcess.Hostname
			} else if rp.Detail == DetailAgentCantConnect {
				kp.Host = rp.AgentProcess.Hostname
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

	// TODO: check tcpproxy config first and then call Get.
	log.Infoln("Getting tcpproxy results...")
	tps, err := f.TCPProxyClient.Get(ctx)
	if err != nil {
		return nil, err
	}
	log.Infof("Got %d tcpproxy conns \n", len(tps))

	rps := f.createRawProcesses(rows, tps)

	f.GetAgentProcesses(ctx, rps)
	ps := f.createKimoProcesses(rps)

	log.Debugf("%d processes are generated \n", len(ps))
	return ps, nil
}
