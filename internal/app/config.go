package app

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/yanet-platform/monalive/internal/announcer"
	"github.com/yanet-platform/monalive/internal/announcer/bird"
	"github.com/yanet-platform/monalive/internal/balancer"
	"github.com/yanet-platform/monalive/internal/balancer/yanet"
	"github.com/yanet-platform/monalive/internal/core"
	"github.com/yanet-platform/monalive/internal/monitoring/xlog"
	"github.com/yanet-platform/monalive/internal/server"
	"github.com/yanet-platform/monalive/pkg/checktun"
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
