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

// RawProcess combines mysql row and agent process
type RawProcess struct {
	MysqlRow     *MysqlRow
	AgentProcess *types.AgentProcess
}

// NewFetcher is constructor fuction for creating a new Fetcher object
func NewFetcher(cfg config.Server) *Fetcher {
	f := new(Fetcher)
	f.MysqlClient = NewMysqlClient(cfg)
	f.TCPProxyClient = NewTCPProxyClient(cfg)
	f.AgentPort = cfg.AgentPort

	return f
}

// GetAgentProcesses gets processes from kimo agents
func (f *Fetcher) GetAgentProcesses(ctx context.Context, rows []*MysqlRow) []*RawProcess {
	log.Infof("Getting processes from %s agents...\n", len(rows))
	var wg sync.WaitGroup
	var rps []*RawProcess
	for _, row := range rows {
		rp := &RawProcess{MysqlRow: row}
		rps = append(rps, rp)

		wg.Add(1)
		go f.getAgentProcess(ctx, &wg, rp)
	}
	wg.Wait()
	log.Infoln("Generating process is done...")

	return rps
}

func (f *Fetcher) getAgentProcess(ctx context.Context, wg *sync.WaitGroup, rp *RawProcess) {
	defer wg.Done()

	ac := NewAgentClient(rp.MysqlRow.Address.IP, f.AgentPort)
	ap, err := ac.Get(ctx, rp.MysqlRow.Address.Port)
	if err != nil {
		log.Debugln(err.Error())
		rp.AgentProcess = &types.AgentProcess{
			Hostname: "ERROR",
		}
	} else {
		rp.AgentProcess = ap
	}
}

func (f *Fetcher) updateProxyFields(rows []*MysqlRow, conns []*TCPProxyConn) {
	log.Infoln("Combining mysql and tcpproxy results...")
	var updated int
	for _, row := range rows {
		conn := findTCPProxyConn(row.Address, conns)
		if conn == nil {
			continue
		}

		updated++

		row.Address.IP = conn.ClientOut.IP
		row.Address.Port = conn.ClientOut.Port
	}
	log.Infof("%d results are updated \n", updated)
}

func (f *Fetcher) createKimoProcesses(rps []*RawProcess) []KimoProcess {
	kps := make([]KimoProcess, 0)
	for _, rp := range rps {
		ut, err := strconv.ParseUint(rp.MysqlRow.Time, 10, 32)
		if err != nil {
			log.Errorf("time %s could not be converted to int", rp.MysqlRow.Time)
		}
		kps = append(kps, KimoProcess{
			ID:        rp.MysqlRow.ID,
			MysqlUser: rp.MysqlRow.User,
			DB:        rp.MysqlRow.DB.String,
			Command:   rp.MysqlRow.Command,
			Time:      uint32(ut),
			State:     rp.MysqlRow.State.String,
			Info:      rp.MysqlRow.Info.String,
			CmdLine:   rp.AgentProcess.CmdLine,
			Pid:       rp.AgentProcess.Pid,
			Host:      rp.AgentProcess.Hostname,
		})
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

	// TODO: check tcpproxy config first and then call Get.
	log.Infoln("Getting tcpproxy results...")
	tps, err := f.TCPProxyClient.Get(ctx)
	if err != nil {
		return nil, err
	}

	f.updateProxyFields(rows, tps)

	rps := f.GetAgentProcesses(ctx, rows)
	ps := f.createKimoProcesses(rps)

	log.Debugf("%d processes are generated \n", len(ps))
	return ps, nil
}
