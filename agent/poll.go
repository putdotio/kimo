package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/log"
	gopsutilNet "github.com/shirou/gopsutil/v4/net"
)

func (a *Agent) pollConns(ctx context.Context) {
	log.Infoln("Polling started...")
	ticker := time.NewTicker(a.Config.PollInterval * time.Second)

	// Initial poll
	if err := a.doPoll(ctx); err != nil {
		log.Errorf("Initial poll failed: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := a.doPoll(ctx); err != nil {
				log.Errorf("Poll failed: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}
func (a *Agent) doPoll(ctx context.Context) error {
	conns, err := getConns(ctx)
	if err != nil {
		return err
	}

	a.Conns = conns

	log.Infof("Updated connections: count=%d", len(conns))
	return nil
}

func getConns(ctx context.Context) ([]gopsutilNet.ConnectionStat, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	type result struct {
		conns []gopsutilNet.ConnectionStat
		err   error
	}

	resultChan := make(chan result, 1)
	go func() {
		// This is an expensive operation.
		// So, we need to call it infrequent to prevent high load on servers those run kimo agents.
		conns, err := gopsutilNet.ConnectionsWithContext(ctx, "all") //todo: all -> tcp
		resultChan <- result{conns, err}

	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("operation timed out: %w", ctx.Err())
	case r := <-resultChan:
		return r.conns, r.err
	}
}
