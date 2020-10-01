package types

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql" // imports mysql driver
	gopsutilNet "github.com/shirou/gopsutil/net"
)

// Addr is an address type that is used to define a host
type Addr struct {
	Host string `json:"host"`
	Port uint32 `json:"port"`
}

// DaemonProcess is the process type in terms of Daemon context
type DaemonProcess struct {
	Laddr    gopsutilNet.Addr `json:"localaddr"`
	Status   string           `json:"status"`
	Pid      int32            `json:"pid"`
	Name     string           `json:"name"`
	Hostname string           `json:"hostname"`
	CmdLine  []string         `json:"cmdline"`
}

// IsEmpty is used for determining whether a DaemonProcess has really a process information or not.
func (dp *DaemonProcess) IsEmpty() bool {
	if dp.Pid > 0 {
		return false
	}
	return true
}

// MysqlProcess is the process type in terms of MySQL context (a row from processlist table)
type MysqlProcess struct {
	ID      int32          `json:"id"`
	User    string         `json:"user"`
	DB      sql.NullString `json:"db"`
	Command string         `json:"command"`
	Time    string         `json:"time"`
	State   sql.NullString `json:"state"`
	Info    sql.NullString `json:"info"`
	Address Addr           `json:"address"`
}

// TCPProxyRecord is type for defining a connection through TCP Proxy to MySQL
type TCPProxyRecord struct {
	ProxyInput   Addr
	ProxyOutput  Addr
	MysqlInput   Addr
	ClientOutput Addr
}

// KimoServerResponse is type for returning a response from kimo server
type KimoServerResponse struct {
	ServerProcesses []ServerProcess `json:"processes"`
}

// ServerProcess is the final processes that is combined from DaemonProcess + TCPProxyRecord + MysqlProcess
type ServerProcess struct {
	ID        int32    `json:"id"`
	MysqlUser string   `json:"mysql_user"`
	DB        string   `json:"db"`
	Command   string   `json:"command"`
	Time      uint32   `json:"time"`
	State     string   `json:"state"`
	Info      string   `json:"info"`
	CmdLine   []string `json:"cmdline"`
	Pid       int32    `json:"pid"`
	Host      string   `json:"host"`
}
