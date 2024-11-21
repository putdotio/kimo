package server

import (
	"context"
	"fmt"
	"kimo/config"
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
	AgentProcess *AgentProcess

	AgentAddress IPPort
}

type AgentProcess struct {
	Address  IPPort
	Response *AgentResponse
	err      error
}

func (ap *AgentProcess) Hostname() string {
	if ap.Response != nil {
		return ap.Response.Hostname
	}

	if ap.err != nil {
		if aErr, ok := ap.err.(*AgentError); ok {
			return aErr.Hostname
		} else {
			return "ERROR"
		}
	}

	return "ERROR" // todo: find a better name.
}

func (rp *RawProcess) Detail() string {
	if rp.TCPProxyConn == nil { // todo: tcpproxy might be absent.
		return "No connection found on tcpproxy"
	}

	if rp.AgentProcess != nil {
		if rp.AgentProcess.err != nil {
			return rp.AgentProcess.err.Error()
		}
	}
	return ""
}

// NewFetcher is constructor fuction for creating a new Fetcher object
func NewFetcher(cfg config.ServerConfig) *Fetcher {
	f := new(Fetcher)
	f.MysqlClient = NewMysqlClient(cfg.MySQL)
	if cfg.TCPProxy.MgmtAddress != "" {
		f.TCPProxyClient = NewTCPProxyClient(cfg.TCPProxy)
	}
	f.AgentPort = cfg.Agent.Port
	return f
}

func (f *Fetcher) getAgentProcess(ctx context.Context, wg *sync.WaitGroup, rp *RawProcess) {
	defer wg.Done()

	ac := NewAgentClient(rp.AgentAddress.IP, f.AgentPort)
	ar, err := ac.Get(ctx, rp.AgentAddress.Port)
	rp.AgentProcess = &AgentProcess{Response: ar, err: err}
}

func addProxyConns(rps []*RawProcess, conns []*TCPProxyConn) {
	log.Infoln("Adding tcpproxy conns...")
	for _, rp := range rps {
		conn := findTCPProxyConn(rp.AgentAddress, conns)
		if conn != nil {
			rp.AgentAddress = IPPort{IP: conn.ClientOut.IP, Port: conn.ClientOut.Port}
			rp.TCPProxyConn = conn
		}
	}
}

func createRawProcesses(rows []*MysqlRow) []*RawProcess {
	log.Infoln("Combining mysql and tcpproxy results...")
	var rps []*RawProcess
	for _, row := range rows {
		rp := &RawProcess{MysqlRow: row}
		rp.AgentAddress = IPPort{IP: row.Address.IP, Port: row.Address.Port}
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

	resultChan := make(chan result, 1)
	go func() {
		rows, err := f.MysqlClient.Get(ctx)
		resultChan <- result{rows, err}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("fetch mysql operation timed out: %w", ctx.Err())
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

	resultChan := make(chan result, 1)
	go func() {
		conns, err := f.TCPProxyClient.Get(ctx)
		resultChan <- result{conns, err}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("fetch tcpproxy operation timed out: %w", ctx.Err())
	case r := <-resultChan:
		return r.conns, r.err
	}
}

func (f *Fetcher) fetchAgents(ctx context.Context, rps []*RawProcess) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	done := make(chan struct{}, 1)
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
		log.Errorf("fetch agents operation timed out: %s", ctx.Err())
		return
	case <-done:
		log.Infoln("All agents are visited.")
		return
	}
}
