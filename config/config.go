package config

import (
	"time"

	"github.com/BurntSushi/toml"
)

// Config is used as config that contains both of agent and server configs
type Config struct {
	Debug  bool `toml:"debug"`
	Agent  Agent
	Server Server
}

// Server is used as server config
type Server struct {
	DSN                    string        `toml:"dsn"`
	AgentPort              uint32        `toml:"agent_port"`
	PollDuration           time.Duration `toml:"poll_duration"`
	TCPProxyMgmtAddress    string        `toml:"tcpproxy_mgmt_address"`
	ListenAddress          string        `toml:"listen_address"`
	AgentConnectTimeout    time.Duration `toml:"agent_connect_timeout"`
	AgentReadTimeout       time.Duration `toml:"agent_read_timeout"`
	TCPProxyConnectTimeout time.Duration `toml:"tcpproxy_connect_timeout"`
	TCPProxyReadTimeout    time.Duration `toml:"tcpproxy_read_timeout"`
}

// Agent is used as anget config on agent machines
type Agent struct {
	ListenAddress string `toml:"listen_address"`
}

// NewConfig is constructor function for Config type
func NewConfig() *Config {
	c := new(Config)
	return c
}

// ReadFile parses a TOML file and returns new Config.
func (c *Config) ReadFile(name string) error {
	_, err := toml.DecodeFile(name, c)
	return err
}
