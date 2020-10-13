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
	ListenAddress string        `toml:"listen_address"`
	PollDuration  time.Duration `toml:"poll_duration"`
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
		DSN:                    "",
		AgentPort:              3333,
		PollDuration:           10,
		TCPProxyMgmtAddress:    "tcpproxy:3307",
		ListenAddress:          "0.0.0.0:3322",
		AgentConnectTimeout:    2,
		AgentReadTimeout:       3,
		TCPProxyConnectTimeout: 1,
		TCPProxyReadTimeout:    1,
	},
	Agent: Agent{
		ListenAddress: "0.0.0.0:3333",
		PollDuration:  30,
	},
	Debug: true,
}
