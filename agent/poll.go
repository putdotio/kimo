package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/log"
	gopsutilNet "github.com/shirou/gopsutil/v4/net"
)

// pollConns periodically checks for connections.
func (a *Agent) pollConns(ctx context.Context) {
	log.Infoln("Polling started...")
	ticker := time.NewTicker(a.Config.PollInterval)

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

// doPoll retrieves the current network connections and updates the Agent's connection state.
// It returns an error if fetching connections fails.
func (a *Agent) doPoll(ctx context.Context) error {
	conns, err := getConns(ctx)
	if err != nil {
		return err
	}

	a.SetConns(conns)

	log.Infof("Updated connections: %d", len(conns))
	return nil
}

// getConns retrieves a list of TCP connections with a 5-second timeout.
// It runs the potentially expensive connection checking operation in a separate goroutine
// to ensure it doesn't block indefinitely.
func getConns(ctx context.Context) ([]gopsutilNet.ConnectionStat, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	type result struct {
		conns []gopsutilNet.ConnectionStat
		err   error
	}

	resultChan := make(chan result, 1)
	go func() {
		// Expensive operation - should be called sparingly to avoid high server load
		conns, err := gopsutilNet.ConnectionsWithContext(ctx, "tcp")
		resultChan <- result{conns, err}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("operation timed out: %w", ctx.Err())
	case r := <-resultChan:
		return r.conns, r.err
	}
}
