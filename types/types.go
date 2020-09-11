package types

import (
	_ "github.com/go-sql-driver/mysql"
	gopsutilNet "github.com/shirou/gopsutil/net"
)

type Addr struct {
	IP   string `json:"ip"`
	Port uint32 `json:"port"`
}

type KimoProcess struct {
	Laddr      gopsutilNet.Addr `json:"localaddr"`
	Status     string           `json:"status"`
	Pid        int32            `json:"pid"`
	Name       string           `json:"name"`
	TcpProxies []Addr           `json:"tcpproxies"`
	Hostname   string           `json:"hostname"`
	CmdLine    string           `json:"cmdline"`
}

type KimoServerResponse struct {
	Hostname      string        `json:"hostname"`
	KimoProcesses []KimoProcess `json:"processes"`
}
