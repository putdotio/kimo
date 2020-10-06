package types

// IPPort is an address type that is used to define a host
type IPPort struct {
	IP   string `json:"ip"`
	Port uint32 `json:"port"`
}

// DaemonProcess is the process type in terms of Daemon context
type DaemonProcess struct {
	Laddr    IPPort   `json:"localaddr"`
	Status   string   `json:"status"`
	Pid      int32    `json:"pid"`
	Name     string   `json:"name"`
	Hostname string   `json:"hostname"`
	CmdLine  []string `json:"cmdline"`
}
