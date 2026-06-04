package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type AWS struct {
	Region      string `yaml:"region"`
	AccountID   string `yaml:"account_id"`
	SSOStartURL string `yaml:"sso_start_url"`
	SSORegion   string `yaml:"sso_region"`
	Profile     string `yaml:"profile"`
	SSHKey      string `yaml:"ssh_key"`
}

type Tunnel struct {
	Name       string `yaml:"name"`
	InstanceID string `yaml:"instance_id"`
	RemotePort int    `yaml:"remote_port"`
	LocalPort  int    `yaml:"local_port"`
	Group      string `yaml:"group"`
	Type       string `yaml:"type"` // tcp | http
}

type Server struct {
	Name       string `yaml:"name"`
	Label      string `yaml:"label"`
	InstanceID string `yaml:"instance_id"`
	TunnelPort int    `yaml:"tunnel_port"`
	SSHUser    string `yaml:"ssh_user"`
	PrivateIP  string `yaml:"private_ip"`
}

type Config struct {
	AWS     AWS      `yaml:"aws"`
	Tunnels []Tunnel `yaml:"tunnels"`
	Servers []Server `yaml:"servers"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parseando %s: %w", path, err)
	}

	if cfg.AWS.Region == "" {
		cfg.AWS.Region = "us-west-1"
	}
	if cfg.AWS.SSHKey == "" {
		cfg.AWS.SSHKey = "~/.ssh/lzb"
	}
	if len(cfg.AWS.SSHKey) >= 2 && cfg.AWS.SSHKey[:2] == "~/" {
		home, _ := os.UserHomeDir()
		cfg.AWS.SSHKey = home + cfg.AWS.SSHKey[1:]
	}
	for i := range cfg.Tunnels {
		if cfg.Tunnels[i].Type == "" {
			cfg.Tunnels[i].Type = "tcp"
		}
		if cfg.Tunnels[i].Group == "" {
			cfg.Tunnels[i].Group = "default"
		}
	}
	for i := range cfg.Servers {
		if cfg.Servers[i].SSHUser == "" {
			cfg.Servers[i].SSHUser = "ubuntu"
		}
	}

	return &cfg, nil
}
