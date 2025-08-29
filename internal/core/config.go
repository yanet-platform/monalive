package core

import (
	"encoding/json"
	"fmt"
	"net/netip"
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
type ConfigLoader func(path string, config *Config) error

// KeepalivedConfigLoader loads a configuration from a keepalived format file.
func KeepalivedConfigLoader(path string, config *Config) error {
	return keepalived.LoadConfig(path, config)
}

// JSONConfigLoader is intended to load a configuration from a JSON file.
// Currently, it is not implemented and will panic if called.
func JSONConfigLoader(path string, config *Config) error {
	// Just read and unmarshal file is not enough.
	// File includes must be supported.
	panic("unimplemented")
}

// Config represents the configuration for virtual servers.
type Config struct {
	// List of virtual servers configurations.
	Services []*service.Config `keepalive:"virtual_server"`
}

// Prepare processes the configuration by performing validation, propagating
// fields, unmapping IP addresses, etc.
func (m *Config) Prepare() error {
	// Prepare each service configuration.
	for _, service := range m.Services {
		if err := service.Prepare(); err != nil {
			return err
		}
	}
	// Validate announce groups.
	m.validateAnnounceGroups()
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

// validateAnnounceGroups ensures that each service in the configuration uses
// the correct announce group for its prefix. It maps prefixes to their
// respective announce groups and ensures consistency across the services.
func (m *Config) validateAnnounceGroups() {
	prefixes := make(map[netip.Prefix]string)
	for _, service := range m.Services {
		if service.AnnounceGroup == "" {
			continue
		}

		prefix := service.Key().Prefix()
		if _, exists := prefixes[prefix]; !exists {
			prefixes[prefix] = service.AnnounceGroup
		}
		service.AnnounceGroup = prefixes[prefix]
	}
}
