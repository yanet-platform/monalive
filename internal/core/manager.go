package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	monalivepb "github.com/yanet-platform/monalive/gen/manager"
)

// ServicesConfig defines the configuration for loading and dumping service
// configurations. It includes the format of the configuration file, the path to
// the configuration file, and the path where the dumped configuration will be
// saved.
type ServicesConfig struct {
	// Format of the services configuration file.
	Format ConfigFormat `yaml:"format"`
	// Path to the services configuration file.
	Path string `yaml:"path"`
	// Path where the dumped configuration will be saved.
	DumpPath string `yaml:"dump_path"`
}

// ManagerConfig encapsulates the services configuration within a manager
// configuration.
type ManagerConfig struct {
	// Embedded ServicesConfig for configuration.
	Services ServicesConfig `yaml:"services_config"`
}

// Manager is a wrapper around the Core to facilitate external communication.
// It handles configuration loading, reloading, and status retrieval.
type Manager struct {
	config   *ManagerConfig
	core     *Core        // core instance that managing all health checking logic
	loader   ConfigLoader // this function is used to load the services configuration
	updateTS time.Time    // last configuration update timestamp
	logger   *slog.Logger
}

// NewManager creates a new Manager instance. It selects the appropriate
// configuration loader based on the format specified in the config.
func NewManager(config *ManagerConfig, core *Core, logger *slog.Logger) (*Manager, error) {
	var loader ConfigLoader
	switch format := config.Services.Format; format {
	case KeepalivedFormat:
		loader = KeepalivedConfigLoader
	case JSONFormat:
		loader = JSONConfigLoader
	default:
		return nil, fmt.Errorf("unknown services configuration format: %s", format)
	}

	return &Manager{
		config: config,
		core:   core,
		loader: loader,
		logger: logger,
	}, nil
}

// Reload handles the RPC method to reload the service configuration.
// Firstly, it loads the configuration using the selected loader.
// Then, it prepares the configuration by validating and processing it.
// Finally, it reloads the Core instance with the new configuration.
//
// Implements the Reload method defined in monalivepb.
func (m *Manager) Reload(ctx context.Context, _ *monalivepb.ReloadRequest) (*monalivepb.ReloadResponse, error) {
	m.logger.Info("starting reload services configuration")
	defer m.logger.Info("reload services configuration finished")

	config, err := m.loadConfig()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := config.Prepare(); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := m.core.Reload(config); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to process reload: %v", err))
	}

	m.updateTS = time.Now()

	if err := config.Dump(m.config.Services.DumpPath); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to dump services config: %v", err))
	}

	return &monalivepb.ReloadResponse{}, nil
}

// GetStatus handles the RPC method to retrieve the current status of the
// service. It implements the GetStatus method defined in monalivepb. It returns
// a [monalivepb.GetStatusResponse] message containing the update timestamp and
// the current status of the services.
func (m *Manager) GetStatus(ctx context.Context, _ *monalivepb.GetStatusRequest) (*monalivepb.GetStatusResponse, error) {
	m.logger.Info("starting retrieve services status")
	defer m.logger.Info("retrieve services status finished")
	return &monalivepb.GetStatusResponse{
		UpdateTimestamp: timestamppb.New(m.updateTS),
		Status:          m.core.Status(),
	}, nil
}

// loadConfig loads the configuration from the specified path using the selected loader function.
// It returns the loaded Config instance or an error if loading fails.
func (m *Manager) loadConfig() (*Config, error) {
	var coreConfig Config
	if err := m.loader(m.config.Services.Path, &coreConfig); err != nil {
		return nil, fmt.Errorf("failed to load services config [format: %s]: %w", m.config.Services.Format, err)
	}
	return &coreConfig, nil
}
