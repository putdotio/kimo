package types

// Addr is an address type that is used to define a host
type Addr struct {
	Host string `json:"host"`
	Port uint32 `json:"port"`
}

// DaemonProcess is the process type in terms of Daemon context
type DaemonProcess struct {
	Laddr    Addr     `json:"localaddr"`
	Status   string   `json:"status"`
	Pid      int32    `json:"pid"`
	Name     string   `json:"name"`
	Hostname string   `json:"hostname"`
	CmdLine  []string `json:"cmdline"`
}
