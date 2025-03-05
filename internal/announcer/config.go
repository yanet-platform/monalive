package announcer

import (
	"time"
)

// Config represents the configuration of the announcer.
type Config struct {
	// The time interval between sending requests to external announcer with
	// prefix updates.
	UpdatePeriod time.Duration `yaml:"update_period"`
	// List of known announce groups.
	AnnounceGroup []string `yaml:"announce_group"`
}

// Default sets the default values for the configuration.
func (m *Config) Default() {
	m.UpdatePeriod = 50 * time.Millisecond
	m.AnnounceGroup = []string{"default"}
}
