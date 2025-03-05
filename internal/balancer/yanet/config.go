package yanet

// Config represents the configuration of YANET load ballancer client.
type Config struct {
	// SockPath is the path to the YANET control plane socket.
	SockPath string `yaml:"control_plane_sock_path"`
}

// Default sets the default values for the configuration.
func (m *Config) Default() {
	m.SockPath = "/var/run/yanet/control_plane.sock"
}
