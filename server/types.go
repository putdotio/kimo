package server

// IPPort is an address type that is used to define a host
type IPPort struct {
	IP   string `json:"ip"`
	Port uint32 `json:"port"`
}
