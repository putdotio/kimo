package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the root configuration structure
type Config struct {
	Debug  bool         `yaml:"debug"`
	Agent  AgentConfig  `yaml:"agent"`
	Server ServerConfig `yaml:"server"`
}

// AgentConfig represents the agent section configuration
type AgentConfig struct {
	ListenAddress string        `yaml:"listen_address"`
	PollInterval  time.Duration `yaml:"poll_interval"`
}

// ServerConfig represents the server section configuration
type ServerConfig struct {
	ListenAddress string        `yaml:"listen_address"`
	PollInterval  time.Duration `yaml:"poll_interval"`
	MySQL         MySQLConfig   `yaml:"mysql"`
	Agent         AgentInfo     `yaml:"agent"`
	TCPProxy      TCPProxy      `yaml:"tcpproxy"`
	Metric        Metric        `yaml:"metric"`
}

// MySQLConfig holds MySQL specific configuration
type MySQLConfig struct {
	DSN string `yaml:"dsn"`
}

// AgentInfo holds agent-related configuration within server section
type AgentInfo struct {
	Port uint32 `yaml:"port"`
}

// TCPProxy holds TCP proxy configuration
type TCPProxy struct {
	MgmtAddress string `yaml:"mgmt_address"`
}

// Metric holds metric-related configuration
type Metric struct {
	CmdlinePatterns []string `yaml:"cmdline_patterns"`
}

// NewConfig is constructor function for Config type
func NewConfig() *Config {
	c := new(Config)
	*c = defaultConfig
	return c
}

// ReadFile parses a yaml config file and loads it into Config object
func (c *Config) LoadConfig(name string) error {
	content, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(content, c)
	if err != nil {
		return fmt.Errorf("parsing YAML file %s: %w", name, err)
	}
	return nil
}

var defaultConfig = Config{
	Debug: true,
	Agent: AgentConfig{
		ListenAddress: "0.0.0.0:3333",
		PollInterval:  10 * time.Second,
	},
	Server: ServerConfig{
		ListenAddress: "0.0.0.0:3322",
		PollInterval:  12 * time.Second,
		MySQL: MySQLConfig{
			DSN: "",
		},
		Agent: AgentInfo{
			Port: 3333,
		},
	},
}
