package server

import (
	"context"
	"strconv"
	"time"

	"github.com/cenkalti/log"
)

func (s *Server) doPoll() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rps, err := s.Fetcher.FetchAll(ctx)
	if err != nil {
		log.Error(err.Error())
		return
	}
	s.KimoProcesses = s.createKimoProcesses(rps)

	log.Debugf("%d processes are set\n", len(s.KimoProcesses))

	s.PrometheusMetric.Set()
}

func (s *Server) pollAgents() {
	ticker := time.NewTicker(s.Config.PollInterval * time.Second)

	for {
		s.doPoll() // poll immediately at initialization
		select {
		// todo: add return case
		case <-ticker.C:
			s.doPoll()
		}
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
