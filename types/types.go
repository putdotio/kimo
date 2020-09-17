package types

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	gopsutilNet "github.com/shirou/gopsutil/net"
)

type Addr struct {
	IP   string `json:"ip"`
	Port uint32 `json:"port"`
}

type DaemonProcess struct {
	Laddr    gopsutilNet.Addr `json:"localaddr"`
	Status   string           `json:"status"`
	Pid      int32            `json:"pid"`
	Name     string           `json:"name"`
	Hostname string           `json:"hostname"`
	CmdLine  string           `json:"cmdline"` // todo: shoudl be array of strings
	Type     string           `json:"type"`    // whether tcpproxy or kimo-server process. todo: should be simple & clean.
}

type MysqlProcess struct {
	ID      int32          `json:"id"`
	User    string         `json:"user"`
	Host    string         `json:"host"`
	Port    uint32         `json:"port"`
	DB      sql.NullString `json:"db"`
	Command string         `json:"command"`
	Time    string         `json:"time"`
	State   sql.NullString `json:"state"`
	Info    sql.NullString `json:"info"`
}

type TcpProxyRecord struct {
	ProxyInput   Addr
	ProxyOutput  Addr
	MysqlInput   Addr
	ClientOutput Addr
}

type KimoProcess struct {
	DaemonProcess   *DaemonProcess
	TcpProxyProcess *DaemonProcess
	MysqlProcess    *MysqlProcess
	TcpProxyRecord  *TcpProxyRecord
}
type KimoDaemonResponse struct {
	Hostname        string          `json:"hostname"`
	DaemonProcesses []DaemonProcess `json:"processes"`
}

type KimoServerResponse struct {
	ServerProcesses []ServerProcess `json:"processes"`
}

type ServerProcess struct {
	ID        int32  `json:"id"`
	MysqlUser string `json:"mysql_user"`
	DB        string `json:"db"`
	Command   string `json:"command"`
	Time      string `json:"time"`
	State     string `json:"state"`
	Info      string `json:"info"`
	CmdLine   string `json:"cmdline"`
	Pid       int32  `json:"pid"`
}
