package balancer

import (
	"time"
)

// Config represents the configuration of the balancer.
type Config struct {
	// FlushPeriod is the time interval between applying new events to the load
	// balancer.
	FlushPeriod time.Duration `yaml:"flush_period"`
	// SyncPeriod is the time interval between requesting the load balancer
	// state.
	SyncPeriod time.Duration `yaml:"sync_states_period"`
}

// Default sets the default values for the configuration.
func (m *Config) Default() {
	m.FlushPeriod = 50 * time.Millisecond
	m.SyncPeriod = 5 * time.Second
}
