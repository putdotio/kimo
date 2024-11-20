package server

import (
	"context"
	"fmt"
	"kimo/config"
	"kimo/types"
	"sync"
	"time"

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
	log.Infoln("Fetching resources...")

	log.Infoln("Fetching mysql rows...")
	rows, err := f.fetchMysql(ctx)
	if err != nil {
		return nil, err
	}
	log.Infof("Got %d mysql rows \n", len(rows))

	rps := createRawProcesses(rows)

	if f.TCPProxyClient != nil {
		log.Infoln("Fetching tcpproxy conns...")
		tps, err := f.fetchTcpProxy(ctx)
		if err != nil {
			return nil, err
		}
		log.Infof("Got %d tcpproxy conns \n", len(tps))
		addProxyConns(rps, tps)
	}

	log.Infof("Fetching %d agents...\n", len(rps))
	f.fetchAgents(ctx, rps)

	log.Debugf("%d raw processes are generated \n", len(rps))
	return rps, nil
}

func (f *Fetcher) fetchMysql(ctx context.Context) ([]*MysqlRow, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	type result struct {
		rows []*MysqlRow
		err  error
	}

	resultChan := make(chan result)
	go func() {
		rows, err := f.MysqlClient.Get(ctx)
		select {
		case resultChan <- result{rows, err}:
			return
		case <-ctx.Done():
			return
		}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("operation timed out: %w", ctx.Err())
	case r := <-resultChan:
		return r.rows, r.err
	}
}

func (f *Fetcher) fetchTcpProxy(ctx context.Context) ([]*TCPProxyConn, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	type result struct {
		conns []*TCPProxyConn
		err   error
	}

	resultChan := make(chan result)
	go func() {
		conns, err := f.TCPProxyClient.Get(ctx)
		select {
		case resultChan <- result{conns, err}:
			return
		case <-ctx.Done():
			return
		}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("operation timed out: %w", ctx.Err())
	case r := <-resultChan:
		return r.conns, r.err
	}
}

func (f *Fetcher) fetchAgents(ctx context.Context, rps []*RawProcess) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	done := make(chan struct{})
	go func() {
		// todo: limit concurrent goroutines.
		var wg sync.WaitGroup
		for _, rp := range rps {
			wg.Add(1)
			go f.getAgentProcess(ctx, &wg, rp)
		}
		wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		log.Errorf("fetchAgents: operation timed out: %s", ctx.Err())
		return
	case <-done:
		log.Infoln("All agents are visited.")
		return
	}
}
