package types

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	gopsutilNet "github.com/shirou/gopsutil/net"
)

type Addr struct {
	Host string `json:"host"`
	Port uint32 `json:"port"`
}

type DaemonProcess struct {
	Laddr    gopsutilNet.Addr `json:"localaddr"`
	Status   string           `json:"status"`
	Pid      int32            `json:"pid"`
	Name     string           `json:"name"`
	Hostname string           `json:"hostname"`
	CmdLine  []string         `json:"cmdline"`
}

func (dp *DaemonProcess) IsEmpty() bool {
	if dp.Pid > 0 {
		return false
	}
	return true
}

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

type TcpProxyRecord struct {
	ProxyInput   Addr
	ProxyOutput  Addr
	MysqlInput   Addr
	ClientOutput Addr
}

type KimoDaemonResponse struct {
	Hostname        string          `json:"hostname"`
	DaemonProcesses []DaemonProcess `json:"processes"`
}

type KimoServerResponse struct {
	ServerProcesses []ServerProcess `json:"processes"`
}

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
	Host      string   `json:"host`
}
