package server

import (
	"context"
	"kimo/config"
	"kimo/types"
	"net"
	"strconv"
	"sync"

	"github.com/cenkalti/log"
)

// Client is used for creating process list
type Client struct {
	MysqlClient    *MysqlClient
	TCPProxyClient *TCPProxyClient

	AgentPort uint32
}

type MysqlProxyResult struct {
	MysqlRow     *MysqlRow
	TCPProxyConn *TCPProxyConn
}

type AgentResult struct {
	*MysqlProxyResult
	AgentProcess *types.AgentProcess
}

// GetAgentProcess get agent processes
func (c *Client) GetAgentProcess(ctx context.Context, wg *sync.WaitGroup, ar *AgentResult) {
	defer wg.Done()
	var host string
	var port uint32

	if ar.MysqlProxyResult.TCPProxyConn != nil {
		host = ar.TCPProxyConn.ClientOut.IP
		port = ar.TCPProxyConn.ClientOut.Port
	} else {
		host = ar.MysqlRow.Address.IP
		port = ar.MysqlRow.Address.Port
	}

	ac := NewAgentClient(host, c.AgentPort)
	ap, err := ac.Get(ctx, port)
	if err != nil {
		log.Debugln(err.Error())
		ar.AgentProcess = &types.AgentProcess{
			Hostname: "ERROR",
		}
	} else {
		ar.AgentProcess = ap
	}
}

// NewClient is constructor fuction for creating a Client object
func NewClient(cfg config.Server) *Client {
	c := new(Client)
	c.MysqlClient = NewMysqlClient(cfg)
	c.TCPProxyClient = NewTCPProxyClient(cfg)
	c.AgentPort = cfg.AgentPort

	return c
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

func findTCPProxyRecord(addr types.IPPort, proxyRecords []*TCPProxyConn) *TCPProxyConn {
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

func (c *Client) combineMysqlAndProxyResults(rows []*MysqlRow, conns []*TCPProxyConn) []*MysqlProxyResult {
	log.Infoln("Combining mysql and tcpproxy results...")
	var mprs []*MysqlProxyResult
	for _, row := range rows {
		conn := findTCPProxyRecord(row.Address, conns)
		if conn == nil {
			continue
		}
		mprs = append(mprs, &MysqlProxyResult{
			MysqlRow:     row,
			TCPProxyConn: conn,
		})
	}
	log.Infof("%d results are combined \n", len(mprs))
	return mprs
}

func (c *Client) getMysqlProxyResult(ctx context.Context) ([]*MysqlProxyResult, error) {
	var mps []*MysqlRow
	var tps []*TCPProxyConn

	errC := make(chan error)
	mpsC := make(chan []*MysqlRow)
	tpsC := make(chan []*TCPProxyConn)

	go c.MysqlClient.Get(ctx, mpsC, errC)
	go c.TCPProxyClient.Get(ctx, tpsC, errC)

	for {
		select {
		case mpsResp := <-mpsC:
			mps = mpsResp
			if tps != nil {
				return c.combineMysqlAndProxyResults(mps, tps), nil
			}
		case tpsResp := <-tpsC:
			tps = tpsResp
			if mps != nil {
				return c.combineMysqlAndProxyResults(mps, tps), nil
			}
		case err := <-errC:
			log.Errorf("Error occured: %s", err.Error())
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *Client) createKimoProcesses(ars []*AgentResult) []KimoProcess {
	kps := make([]KimoProcess, 0)
	for _, ar := range ars {
		ut, err := strconv.ParseUint(ar.MysqlRow.Time, 10, 32)
		if err != nil {
			log.Errorf("time %s could not be converted to int", ar.MysqlRow.Time)
		}
		kps = append(kps, KimoProcess{
			ID:        ar.MysqlRow.ID,
			MysqlUser: ar.MysqlRow.User,
			DB:        ar.MysqlRow.DB.String,
			Command:   ar.MysqlRow.Command,
			Time:      uint32(ut),
			State:     ar.MysqlRow.State.String,
			Info:      ar.MysqlRow.Info.String,
			CmdLine:   ar.AgentProcess.CmdLine,
			Pid:       ar.AgentProcess.Pid,
			Host:      ar.AgentProcess.Hostname,
		})
	}
	return kps
}

// FetchAll is used to create processes from mysql to agents
func (c *Client) FetchAll(ctx context.Context) ([]KimoProcess, error) {
	log.Debugf("Fetching...")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mysqlProxyResult, err := c.getMysqlProxyResult(ctx)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	log.Infof("Getting processes from %s agents...\n", len(mysqlProxyResult))
	var wg sync.WaitGroup
	var ars []*AgentResult
	for _, mpr := range mysqlProxyResult {
		ar := &AgentResult{MysqlProxyResult: mpr}
		ars = append(ars, ar)

		wg.Add(1)
		go c.GetAgentProcess(ctx, &wg, ar)
	}
	wg.Wait()
	log.Infoln("Generating process is done...")

	ps := c.createKimoProcesses(ars)

	log.Debugf("%d processes are generated \n", len(ps))
	return ps, nil
}
