package config

import "github.com/BurntSushi/toml"

type Config struct {
	Debug  bool `toml:"debug"`
	Client Client
	Server Server
}

type Client struct {
	DSN                 string `toml:"dsn"`
	ServerPort          uint32 `toml:"server_port"`
	TcpProxyMgmtAddress string `toml:"tcpproxy_mgmt_address"`
}

type Server struct {
	ListenAddress string `toml:"listen_address"`
}

func NewConfig() *Config {
	c := new(Config)
	return c
}

// ReadFile parses a TOML file and returns new Config.
func (c *Config) ReadFile(name string) error {
	_, err := toml.DecodeFile(name, c)
	return err
}
