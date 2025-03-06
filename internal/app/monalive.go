package app

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"monalive/internal/announcer"
	"monalive/internal/announcer/bird"
	"monalive/internal/balancer"
	"monalive/internal/balancer/yanet"
	"monalive/internal/core"
	"monalive/internal/monitoring/xlog"
	"monalive/internal/server"
	"monalive/internal/utils/xtls"
	"monalive/pkg/checktun"
)

type Monalive struct {
	config Config

	announcer *announcer.Announcer
	balancer  *balancer.Balancer
	tunneler  *checktun.CheckTun

	core        *core.Core
	coreManager *core.Manager

	server *server.Server

	logger *slog.Logger
}

// New creates a new instance of Monalive service.
func New(config Config, logger *slog.Logger) (*Monalive, error) {
	// Set the minimum TLS version from the configuration.
	if err := xtls.SetTLSMinVersion(config.TLSMinVersion); err != nil {
		logger.Warn("failed to set TLSMinVersion from config", slog.Any("error", err))
	}

	// Initialize the Bird announcer.
	bird, err := bird.New(config.Bird, config.Announcer.AnnounceGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to create bird: %w", err)
	}

	// Create an announcer instance.
	announcer := announcer.New(config.Announcer, bird, logger)

	// Initialize the YANET client to communicate with YANET control plane.
	yanetClient, err := yanet.NewClient(config.YANET)
	if err != nil {
		return nil, fmt.Errorf("failed to create yanet client: %w", err)
	}

	// Create a balancer worker instance.
	balancer := balancer.New(config.Balancer, yanetClient, logger)

	// Initialize the check tunneler.
	tunneler := checktun.New(config.Tunnel, xlog.NewNopLogger())

	// Create the core service and its manager.
	coreService := core.New(announcer, balancer, logger)
	coreManager, err := core.NewManager(config.Service, coreService, logger)
	if err != nil {
		return nil, err
	}

	// Initialize the server with the core manager.
	server := server.New(config.Server, coreManager)

	return &Monalive{
		config:      config,
		announcer:   announcer,
		balancer:    balancer,
		tunneler:    tunneler,
		core:        coreService,
		coreManager: coreManager,
		server:      server,
		logger:      logger,
	}, nil
}

// Run starts all components of the Monalive service and manages their
// lifecycle.
func (m *Monalive) Run(ctx context.Context) error {
	// Create an errgroup with a derived context for managing goroutines.
	wg, ctx := errgroup.WithContext(ctx)

	wg.Go(func() error {
		return m.announcer.Run()
	})

	wg.Go(func() error {
		return m.balancer.Run(ctx)
	})

	wg.Go(func() error {
		return m.tunneler.Run(ctx)
	})

	wg.Go(func() error {
		return m.server.Run(ctx)
	})

	wg.Go(func() error {
		m.core.Run(ctx)
		return nil
	})

	if _, err := m.coreManager.Reload(ctx, nil); err != nil {
		m.logger.Error("failed to reload core service", slog.Any("error", err))
	}

	// Handle graceful shutdown when the context is cancelled.
	wg.Go(func() error {
		<-ctx.Done()

		// Stop all components gracefully.

		m.server.Stop()

		// It is important that the core is stopped before the balancer.
		// Otherwise, the load balancer state will keep false enabled reals.
		m.core.Stop()
		m.balancer.Stop()

		m.announcer.Stop()
		m.tunneler.Stop()

		return ctx.Err()
	})

	// Wait for all goroutines in the errgroup to complete.
	return wg.Wait()
}
