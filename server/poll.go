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
	ticker := time.NewTicker(s.Config.PollInterval * time.Second)

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
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

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
		s.KimoProcesses = s.createKimoProcesses(r.rps)
		s.PrometheusMetric.Set()
		log.Debugf("%d processes are set\n", len(s.KimoProcesses))
		return nil
	}

}

func (s *Server) createKimoProcesses(rps []*RawProcess) []KimoProcess {
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
			if rp.Details != nil {
				kp.Detail = rp.Detail()
			}
		}
		kps = append(kps, kp)
	}
	return kps
}
