package agent

import (
	"time"

	"github.com/cenkalti/log"
	gopsutilNet "github.com/shirou/gopsutil/v4/net"
)

func (a *Agent) pollConns() {
	// todo: run with context
	log.Debugln("Polling...")
	ticker := time.NewTicker(a.Config.PollInterval * time.Second)

	for {
		a.getConns() // poll immediately at the initialization
		select {
		// todo: add return case
		case <-ticker.C:
			a.getConns()
		}
	}

}
func (a *Agent) getConns() {
	// This is an expensive operation.
	// So, we need to call it infrequent to prevent high load on servers those run kimo agents.
	conns, err := gopsutilNet.Connections("all")
	if err != nil {
		log.Errorln(err.Error())
		return
	}
	a.Conns = conns
}
