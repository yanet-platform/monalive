package server

// Config represents the configuration of the Server.
type Config struct {
	// Address for the HTTP server to listen on.
	HTTPAddr string `yaml:"http_addr"`
	// Address for the gRPC server to listen on.
	GRPCAddr string `yaml:"grpc_addr"`
}

// Default sets the default values for the configuration.
func (m *Config) Default() {
	m.HTTPAddr = "[::1]:14080"
	m.GRPCAddr = "[::1]:14081"
}
