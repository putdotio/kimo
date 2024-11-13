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
	Mysql    *Mysql
	TCPProxy *TCPProxy
	Agent    *Agent
}

// KimoProcess is combined with processes from mysql to agent through tcpproxy
type KimoProcess struct {
	AgentProcess *types.AgentProcess
	MysqlRow     *MysqlRow
	TCPProxyConn *TCPProxyConn
	Agent        *Agent
}

type MysqlResult struct {
	MysqlRows     []*MysqlRow
	TCPProxyConns []*TCPProxyConn
}

// SetAgentProcess is used to set agent process of a KimoProcess
func (kp *KimoProcess) SetAgentProcess(ctx context.Context, wg *sync.WaitGroup) {
	// todo: get rid of this function.
	defer wg.Done()
	var host string
	var port uint32

	if kp.TCPProxyConn != nil {
		host = kp.TCPProxyConn.ClientOut.IP
		port = kp.TCPProxyConn.ClientOut.Port
	} else {
		host = kp.MysqlRow.Address.IP
		port = kp.MysqlRow.Address.Port
	}
	ap, err := kp.Agent.Fetch(ctx, host, port)
	if err != nil {
		log.Debugln(err.Error())
		kp.AgentProcess = &types.AgentProcess{
			Hostname: "ERROR",
		}
	} else {
		kp.AgentProcess = ap
	}
}

// NewClient is constructor fuction for creating a Client object
func NewClient(cfg config.Server) *Client {
	p := new(Client)
	p.Mysql = NewMysql(cfg)
	p.TCPProxy = NewTCPProxy(cfg)
	p.Agent = NewAgent(cfg)
	return p
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

func (c *Client) initializeKimoProcesses(mps []*MysqlRow, tps []*TCPProxyConn) []*KimoProcess {
	log.Infoln("Initializing Kimo processes...")
	var kps []*KimoProcess
	for _, mp := range mps {
		tpr := findTCPProxyRecord(mp.Address, tps)
		if tpr == nil {
			continue
		}
		kps = append(kps, &KimoProcess{
			MysqlRow:     mp,
			TCPProxyConn: tpr,
			Agent:        c.Agent,
		})
	}
	log.Infof("%d processes are initialized \n", len(kps))
	return kps
}

func (c *Client) getMysqlResult(ctx context.Context) (*MysqlResult, error) {
	var mps []*MysqlRow
	var tps []*TCPProxyConn

	errC := make(chan error)
	mpsC := make(chan []*MysqlRow)
	tpsC := make(chan []*TCPProxyConn)

	go c.Mysql.Get(ctx, mpsC, errC)
	go c.TCPProxy.Get(ctx, tpsC, errC)

	for {
		select {
		case mpsResp := <-mpsC:
			mps = mpsResp
			if tps != nil {
				return &MysqlResult{MysqlRows: mps, TCPProxyConns: tps}, nil
			}
		case tpsResp := <-tpsC:
			tps = tpsResp
			if mps != nil {
				return &MysqlResult{MysqlRows: mps, TCPProxyConns: tps}, nil
			}
		case err := <-errC:
			log.Errorf("Error occured: %s", err.Error())
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *Client) setAgentProcesses(ctx context.Context, kps []*KimoProcess) {
	log.Infof("Generating %d kimo processes...\n", len(kps))
	var wg sync.WaitGroup
	for _, kp := range kps {
		wg.Add(1)
		go kp.SetAgentProcess(ctx, &wg)
	}
	wg.Wait()
	log.Infoln("Generating process is done...")
}

func (c *Client) createProcesses(kps []*KimoProcess) []Process {
	ps := make([]Process, 0)
	for _, kp := range kps {
		ut, err := strconv.ParseUint(kp.MysqlRow.Time, 10, 32)
		if err != nil {
			log.Errorf("time %s could not be converted to int", kp.MysqlRow.Time)
		}
		ps = append(ps, Process{
			ID:        kp.MysqlRow.ID,
			MysqlUser: kp.MysqlRow.User,
			DB:        kp.MysqlRow.DB.String,
			Command:   kp.MysqlRow.Command,
			Time:      uint32(ut),
			State:     kp.MysqlRow.State.String,
			Info:      kp.MysqlRow.Info.String,
			CmdLine:   kp.AgentProcess.CmdLine,
			Pid:       kp.AgentProcess.Pid,
			Host:      kp.AgentProcess.Hostname,
		})
	}
	return ps
}

// FetchAll is used to create processes from mysql to agents
func (c *Client) FetchAll(ctx context.Context) ([]Process, error) {
	log.Debugf("Fetching...")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mysqlResult, err := c.getMysqlResult(ctx)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	kps := c.initializeKimoProcesses(mysqlResult.MysqlRows, mysqlResult.TCPProxyConns)

	c.setAgentProcesses(ctx, kps)
	ps := c.createProcesses(kps)

	log.Debugf("%d processes are generated \n", len(ps))
	return ps, nil
}
