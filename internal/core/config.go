package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yanet-platform/monalive/internal/core/service"
	"github.com/yanet-platform/monalive/pkg/keepalived"
)

// ConfigFormat represents the format in which the services configuration is
// loaded.
type ConfigFormat string

const (
	// KeepalivedFormat represents the keepalived configuration format.
	KeepalivedFormat ConfigFormat = "keepalived"
	// JSONFormat represents the JSON configuration format.
	JSONFormat ConfigFormat = "json" // currently unused
)

// ConfigLoader is a function type for loading configuration.
type ConfigLoader func(path string) (*Config, error)

// KeepalivedConfigLoader loads a configuration from a keepalived format file.
func KeepalivedConfigLoader(path string) (*Config, error) {
	keepalivedConfig, err := keepalived.LoadConfig(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load keepalived config: %w", err)
	}

	// Convert the keepalived services configuration to the internal
	// representation.
	services := make([]*service.Config, len(keepalivedConfig.Services))
	for i, s := range keepalivedConfig.Services {
		services[i] = (*service.Config)(s)
	}

	config := &Config{
		Services: services,
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid services config: %w", err)
	}
	return config, nil
}

// JSONConfigLoader is intended to load a configuration from a JSON file.
// Currently, it is not implemented and will panic if called.
func JSONConfigLoader(path string) (*Config, error) {
	// Just read and unmarshal file is not enough.
	// File includes must be supported.
	panic("unimplemented")
}

// Config represents the configuration for virtual servers.
type Config struct {
	// List of virtual servers configurations.
	Services []*service.Config
}

// Validate checks Monalive-specific constraints on the services configuration.
func (m *Config) Validate() error {
	for _, service := range m.Services {
		for _, real := range service.Reals {
			count := len(real.TCPCheckers) + len(real.HTTPCheckers) + len(real.HTTPSCheckers) + len(real.GRPCCheckers)
			if count > 1 {
				return fmt.Errorf("real_server %s (port %q): at most one checker is supported, got %d", real.IP, real.Port, count)
			}
		}
	}
	return nil
}

// Dump serializes the configuration to a JSON file at the specified path. It
// creates a temporary file to ensure atomic write operations.
func (m *Config) Dump(path string) error {
	// Marshal the services configuration to JSON with indentation.
	jsonCfg, err := json.MarshalIndent(m.Services, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal services config: %w", err)
	}

	// Create a temporary file to write the configuration.
	tmpFile, err := os.CreateTemp(filepath.Split(path))
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	// Write the JSON configuration to the temporary file.
	if _, err := tmpFile.Write(jsonCfg); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}

	// Ensure the file is written to disk.
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}

	// Close the temporary file.
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close: %w", err)
	}

	// Rename the temporary file to the target path.
	if err := os.Rename(tmpFile.Name(), path); err != nil {
		return fmt.Errorf("failed to move: %w", err)
	}

	return nil
}
