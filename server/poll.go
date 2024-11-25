package server

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/cenkalti/log"
)

func (s *Server) pollAgents(ctx context.Context) {
	log.Infoln("Polling started...")
	ticker := time.NewTicker(s.Config.PollInterval)

	// Initial poll
	if err := s.doPoll(ctx); err != nil {
		log.Errorf("Initial poll failed: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := s.doPoll(ctx); err != nil {
				log.Errorf("Poll failed: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Server) doPoll(ctx context.Context) error {
	type result struct {
		rps []*RawProcess
		err error
	}

	resultChan := make(chan result)

	go func() {
		rps, err := s.Fetcher.FetchAll(ctx)
		select {
		case resultChan <- result{rps, err}:
			return
		case <-ctx.Done():
			return
		}
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("operation timed out while fetching all: %w", ctx.Err())
	case r := <-resultChan:
		if r.err != nil {
			return r.err
		}
		s.SetProcesses(createKimoProcesses(r.rps))
		s.PrometheusMetric.Set(s.GetProcesses())
		log.Debugf("%d processes are set\n", len(s.GetProcesses()))
		return nil
	}

}

func createKimoProcesses(rps []*RawProcess) []KimoProcess {
	kps := make([]KimoProcess, 0)
	for _, rp := range rps {
		ut, err := strconv.ParseUint(rp.MysqlRow.Time, 10, 32)
		if err != nil {
			log.Errorf("time %s could not be converted to int", rp.MysqlRow.Time)
		}
		var kp KimoProcess

		// set mysql properties
		kp.ID = rp.MysqlRow.ID
		kp.MysqlUser = rp.MysqlRow.User
		kp.DB = rp.MysqlRow.DB.String
		kp.Command = rp.MysqlRow.Command
		kp.Time = uint32(ut)
		kp.State = rp.MysqlRow.State.String
		kp.Info = rp.MysqlRow.Info.String

		// set agent process properties
		kp.CmdLine = rp.AgentProcess.CmdLine()
		kp.ConnectionStatus = rp.AgentProcess.ConnectionStatus()
		kp.Pid = rp.AgentProcess.Pid()
		kp.Host = rp.AgentProcess.Hostname()
		kp.Detail = rp.Detail()

		kps = append(kps, kp)
	}
	return kps
}
