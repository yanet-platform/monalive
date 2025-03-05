package app

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"

	"monalive/internal/announcer"
	"monalive/internal/announcer/bird"
	"monalive/internal/balancer"
	"monalive/internal/balancer/yanet"
	"monalive/internal/core"
	"monalive/internal/monitoring/xlog"
	"monalive/internal/server"
	"monalive/pkg/checktun"
)

type Config struct {
	Logger *xlog.Config `yaml:"logging"`

	Balancer *balancer.Config `yaml:"balancer"`
	YANET    *yanet.Config    `yaml:"yanet"`

	Announcer *announcer.Config `yaml:"announcer"`
	Bird      *bird.Config      `yaml:"bird"`

	TLSMinVersion string              `yaml:"tls_min_version"`
	Service       *core.ManagerConfig `yaml:"service"`
	Server        *server.Config      `yaml:"server"`
	Tunnel        checktun.Config     `yaml:"check_tun"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err = yaml.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return config, nil
}
