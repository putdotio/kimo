package server

import (
	"context"
	"strconv"
	"time"

	"github.com/cenkalti/log"
)

func (s *Server) pollAgents(ctx context.Context) {
	log.Infoln("Polling...")
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
	rps, err := s.Fetcher.FetchAll(ctx)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	s.KimoProcesses = s.createKimoProcesses(rps)

	log.Debugf("%d processes are set\n", len(s.KimoProcesses))

	s.PrometheusMetric.Set()
	return nil
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
