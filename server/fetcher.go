package server

import (
	"context"
	"fmt"
	"kimo/config"
	"sync"
	"time"

	"github.com/cenkalti/log"
)

// Fetcher fetches process info(s) from resources
type Fetcher struct {
	MysqlClient    *MysqlClient
	TCPProxyClient *TCPProxyClient

	AgentListenPort uint32
}

// RawProcess combines resources information(mysql row, tcp proxy conn, agent process etc.)
type RawProcess struct {
	MysqlRow     *MysqlRow
	TCPProxyConn *TCPProxyConn
	AgentProcess *AgentProcess

	TCPProxyEnabled bool
}

// AgentAddress returns agent address considering proxy usage.
func (rp *RawProcess) AgentAddress() IPPort {
	if rp.TCPProxyConn != nil {
		return IPPort{IP: rp.TCPProxyConn.ClientOut.IP, Port: rp.TCPProxyConn.ClientOut.Port}
	}
	return IPPort{IP: rp.MysqlRow.Address.IP, Port: rp.MysqlRow.Address.Port}
}

// AgentHost returns agent's host
func (rp *RawProcess) AgentHost() string {
	if rp.AgentProcess != nil {
		return rp.AgentProcess.Host()
	}
	return rp.AgentAddress().IP
}

// Detail returns error detail of the process.
func (rp *RawProcess) Detail() string {
	if rp.TCPProxyEnabled && rp.TCPProxyConn == nil {
		return "No connection found on tcpproxy"
	}

	if rp.AgentProcess != nil {
		if rp.AgentProcess.err != nil {
			return rp.AgentProcess.err.Error()
		}
	}
	return ""
}

// NewFetcher creates and returns a new Fetcher.
func NewFetcher(cfg config.ServerConfig) *Fetcher {
	f := new(Fetcher)
	f.MysqlClient = NewMysqlClient(cfg.MySQL)
	if cfg.TCPProxy.MgmtAddress != "" {
		f.TCPProxyClient = NewTCPProxyClient(cfg.TCPProxy)
	}
	f.AgentListenPort = cfg.Agent.Port
	return f
}

// createRawProcesses creates raw process and inserts given param.
func createRawProcesses(rows []*MysqlRow) []*RawProcess {
	log.Debugln("Creating raw processes...")
	var rps []*RawProcess
	for _, row := range rows {
		rp := &RawProcess{MysqlRow: row}
		rps = append(rps, rp)
	}
	return rps
}

// addProxyConns adds TCPProxy connection info if TCPProxy is enabled.
func addProxyConns(rps []*RawProcess, conns []*TCPProxyConn) {
	log.Debugln("Adding tcpproxy conns...")
	for _, rp := range rps {
		conn := findTCPProxyConn(rp.AgentAddress(), conns)
		if conn != nil {
			rp.TCPProxyConn = conn
		}
	}
}
func addAgentProcesses(rps []*RawProcess, aps []*AgentProcess) {
	log.Debugln("Adding agent processes...")
	for _, rp := range rps {
		ap := findAgentProcess(rp.AgentAddress(), aps)
		if ap != nil {
			rp.AgentProcess = ap
		}
	}
}

// FetchAll fetches and creates processes from resources to agents
func (f *Fetcher) FetchAll(ctx context.Context) ([]*RawProcess, error) {
	log.Debugln("Fetching resources...")

	log.Debugln("Fetching mysql rows...")
	rows, err := f.fetchMysql(ctx)
	if err != nil {
		return nil, err
	}
	log.Debugf("Got %d mysql rows \n", len(rows))

	rps := createRawProcesses(rows)

	if f.TCPProxyClient != nil {
		for _, rp := range rps {
			rp.TCPProxyEnabled = true
		}

		log.Debugln("Fetching tcpproxy conns...")
		tps, err := f.fetchTcpProxy(ctx)
		if err != nil {
			return nil, err
		}
		log.Debugf("Got %d tcpproxy conns \n", len(tps))
		addProxyConns(rps, tps)
	}

	log.Debugln("Fetching agents...")
	aps := f.fetchAgents(ctx, rps)
	log.Debugf("Got %d agent processes \n", len(aps))

	addAgentProcesses(rps, aps)

	log.Debugf("%d raw processes are generated \n", len(rps))
	return rps, nil
}

// fetchMysql retrieves MySQL data with timeout.
// It performs the fetch operation in a separate goroutine to prevent blocking.
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

// fetchTcpProxy retrieves TCP proxy connections with timeout.
// It performs the fetch operation in a separate goroutine to prevent blocking.
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

// fetchAgents concurrently retrieves process information from multiple agents with timeout.
func (f *Fetcher) fetchAgents(ctx context.Context, rps []*RawProcess) []*AgentProcess {
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	type result struct {
		response []*AgentResponse
		err      error
		IP       string // todo: fix ambiguity.
	}

	agentIPPorts := make(map[string][]uint32)
	for _, rp := range rps {
		addr := rp.AgentAddress()
		agentIPPorts[addr.IP] = append(agentIPPorts[addr.IP], addr.Port)
	}

	done := make(chan struct{}, 1)
	resultChan := make(chan *result, len(agentIPPorts))

	// Get results from agents
	go func() {
		// todo: limit concurrent goroutines.
		var wg sync.WaitGroup
		for agentIP, ports := range agentIPPorts {
			wg.Add(1)

			agentAddr := IPPort{IP: agentIP, Port: f.AgentListenPort}
			go func(address IPPort, ports []uint32) {
				defer wg.Done()

				ac := NewAgentClient(address)
				ar, err := ac.Get(ctx, ports)
				resultChan <- &result{response: ar, err: err, IP: address.IP}
			}(agentAddr, ports)
		}
		wg.Wait()
		close(resultChan) // Close channel to signal no more results
		done <- struct{}{}
	}()

	// Collect results
	var aps []*AgentProcess

	// Use a separate goroutine to collect results
	apsDone := make(chan struct{})
	go func() {
		for result := range resultChan {
			if result != nil {
				for _, resp := range result.response {
					ap := NewAgentProcesss(resp, result.IP, resp.Port, result.err)
					aps = append(aps, ap)
				}
			}
		}
		close(apsDone) // close channel to signal result collection is done.
	}()

	// Wait for either completion or timeout
	select {
	case <-ctx.Done():
		log.Errorf("fetch agents operation timed out: %s", ctx.Err())
		return aps
	case <-done:
		// Wait for all results to be collected
		<-apsDone
		log.Debugf("All agents are visited. %d processes found \n", len(aps))
		return aps
	}
}
