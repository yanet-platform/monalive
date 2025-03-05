package bird

// Config represents the configuration of the BIRD client.
type Config struct {
	// BatchSize determines the maximum number of messages sent to the BIRD in a
	// single request. This value should not exceed the value set in the BIRD
	// daemon.
	BatchSize int `yaml:"batch_size"`
	// SockDir is the directory where the BIRD UNIX domain sockets are located.
	SockDir string `yaml:"sock_dir"`
}

// Default sets the default values for the configuration.
func (m *Config) Default() {
	m.BatchSize = 4096
	m.SockDir = "/var/run"
}
