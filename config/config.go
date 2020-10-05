package config

import (
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Debug  bool `toml:"debug"`
	Daemon Daemon
	Server Server
}

type Server struct {
	DSN                    string        `toml:"dsn"`
	DaemonPort             uint32        `toml:"daemon_port"`
	TCPProxyMgmtAddress    string        `toml:"tcpproxy_mgmt_address"`
	ListenAddress          string        `toml:"listen_address"`
	DaemonConnectTimeout   time.Duration `toml:"daemon_connect_timeout"`
	DaemonReadTimeout      time.Duration `toml:"daemon_read_timeout"`
	TCPProxyConnectTimeout time.Duration `toml:"tcpproxy_connect_timeout"`
	TCPProxyReadTimeout    time.Duration `toml:"tcpproxy_read_timeout"`
}

type Daemon struct {
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
