package keepalived

import (
	"fmt"
	"net/netip"
)

// Config represents the configuration for virtual servers.
type Config struct {
	// List of virtual servers configurations.
	Services []*Service `keepalive:"virtual_server"`
}

func LoadConfig(path string) (*Config, error) {
	cfgRoot, err := parseFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	config := &Config{}
	if err := configDecode(config, cfgRoot); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	if err := config.Prepare(); err != nil {
		return nil, fmt.Errorf("failed to prepare config: %w", err)
	}

	return config, nil
}

// Prepare processes the configuration by performing validation, propagating
// fields, unmapping IP addresses, etc.
func (m *Config) Prepare() error {
	// Prepare each service configuration.
	for _, service := range m.Services {
		if err := service.Prepare(); err != nil {
			return fmt.Errorf("failed to prepare keepalived service: %w", err)
		}
	}
	// Validate announce groups.
	m.validateAnnounceGroups()
	return nil
}

// validateAnnounceGroups ensures that each service in the configuration uses
// the correct announce group for its prefix. It maps prefixes to their
// respective announce groups and ensures consistency across the services.
func (m *Config) validateAnnounceGroups() {
	prefixes := make(map[netip.Prefix]string)
	for _, service := range m.Services {
		if service.AnnounceGroup == "" {
			continue
		}

		prefix := netip.PrefixFrom(service.VIP, service.VIP.BitLen())
		if _, exists := prefixes[prefix]; !exists {
			prefixes[prefix] = service.AnnounceGroup
		}
		service.AnnounceGroup = prefixes[prefix]
	}
}
