package server

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/log"
)

// pollAgents continuously polls for agent information at configured intervals.
// It performs an initial poll immediately, then polls based on PollInterval from config.
func (s *Server) pollAgents(ctx context.Context) error {
	log.Infoln("Polling started...")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ticker := time.NewTicker(s.Config.PollInterval)

	// Initial poll
	if err := s.doPoll(ctx); err != nil {
		log.Errorf("Initial poll failed: %v", err)
		return err
	}

	for {
		select {
		case <-ticker.C:
			if err := s.doPoll(ctx); err != nil {
				log.Errorf("Poll failed: %v", err)
			}
		case <-ctx.Done():
			log.Infoln("Polling stopped.")
			return nil
		}
	}
}

// doPoll performs a single polling operation to fetch and update process information.
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
			log.Infoln("FetchAll stopped.")
			return
		}
	}()

	select {
	case <-ctx.Done():
		err := fmt.Errorf("doPoll operation stopped: %w", ctx.Err())
		s.UpdateHealth(err)
		return err
	case r := <-resultChan:
		if r.err != nil {
			s.UpdateHealth(r.err)
			return r.err
		}
		kps := s.ConvertProcesses(r.rps)
		s.SetProcesses(kps)
		s.PrometheusMetric.Set(s.GetProcesses())
		s.UpdateHealth(nil)
		log.Debugf("%d processes are set\n", len(s.GetProcesses()))
		return nil
	}
}
