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
	DSN                 string        `toml:"dsn"`
	AgentPort           uint32        `toml:"agent_port"`
	PollInterval        time.Duration `toml:"poll_interval"`
	TCPProxyMgmtAddress string        `toml:"tcpproxy_mgmt_address"`
	ListenAddress       string        `toml:"listen_address"`
}

// Agent is used as anget config on agent machines
type Agent struct {
	ListenAddress string        `toml:"listen_address"`
	PollInterval  time.Duration `toml:"poll_interval"`
}

// NewConfig is constructor function for Config type
func NewConfig() *Config {
	c := new(Config)
	*c = defaultConfig
	return c
}

// ReadFile parses a TOML file and returns new Config.
func (c *Config) ReadFile(name string) error {
	_, err := toml.DecodeFile(name, c)
	return err
}

var defaultConfig = Config{
	Server: Server{
		DSN:           "",
		AgentPort:     3333,
		PollInterval:  10,
		ListenAddress: "0.0.0.0:3322",
	},
	Agent: Agent{
		ListenAddress: "0.0.0.0:3333",
		PollInterval:  30,
	},
	Debug: true,
}
