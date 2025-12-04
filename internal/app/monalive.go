package app

import (
	"context"
	"fmt"
	"time"

	log "go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/yanet-platform/monalive/internal/announcer"
	"github.com/yanet-platform/monalive/internal/announcer/bird"
	"github.com/yanet-platform/monalive/internal/balancer"
	"github.com/yanet-platform/monalive/internal/balancer/yanet"
	"github.com/yanet-platform/monalive/internal/core"
	"github.com/yanet-platform/monalive/internal/monitoring/metrics"
	"github.com/yanet-platform/monalive/internal/monitoring/metrics/prometheus"
	"github.com/yanet-platform/monalive/internal/server"
	"github.com/yanet-platform/monalive/internal/utils/xtls"
	"github.com/yanet-platform/monalive/pkg/checktun"
)

type Monalive struct {
	config Config

	announcer *announcer.Announcer
	balancer  *balancer.Balancer
	tunneler  *checktun.CheckTun

	core        *core.Core
	coreManager *core.Manager

	server *server.Server

	metrics *metrics.ScopedMetrics
	logger  *log.Logger
}

// New creates a new instance of Monalive service.
func New(config Config, logger *log.Logger) (*Monalive, error) {
	// Set the minimum TLS version from the configuration.
	if err := xtls.SetTLSMinVersion(config.TLSMinVersion); err != nil {
		logger.Warn("failed to set TLSMinVersion from config", log.Error(err))
	}

	scopedMetrics := metrics.NewScopedMetrics(logger)
	scopedMetrics.Set(
		metrics.Global,
		prometheus.NewProvider(logger.With(log.String("scope", string(metrics.Global)))),
	)
	if config.Metrics.EnablePerServiceMetrics {
		scopedMetrics.Set(
			metrics.PerService,
			prometheus.NewProvider(logger.With(log.String("scope", string(metrics.PerService)))),
		)
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
	balancer := balancer.New(config.Balancer, yanetClient, announcer, logger)

	// Initialize the check tunneler.
	tunneler, err := checktun.New(config.Tunnel, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create checktun: %w", err)
	}

	// Create the core service and its manager.
	coreService := core.New(announcer, balancer, scopedMetrics, logger)
	coreManager, err := core.NewManager(config.Service, coreService, scopedMetrics.Scope(metrics.Global), logger)
	if err != nil {
		return nil, err
	}

	// Initialize the server with the core manager.
	server := server.New(config.Server, scopedMetrics.Gatherers(), coreManager, logger)

	return &Monalive{
		config:      config,
		announcer:   announcer,
		balancer:    balancer,
		tunneler:    tunneler,
		core:        coreService,
		coreManager: coreManager,
		server:      server,
		metrics:     scopedMetrics,
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
		m.logger.Error("failed to reload core service", log.Error(err))
	}

	// Handle graceful shutdown when the context is cancelled.
	wg.Go(func() error {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Stop all components gracefully.

		m.server.Stop()

		// It is important that the core is stopped before the balancer.
		// Otherwise, the load balancer state will keep false enabled reals.
		m.core.Stop()
		m.balancer.Stop()

		m.announcer.Stop()
		m.tunneler.Stop()

		m.metrics.Shutdown(shutdownCtx)

		return ctx.Err()
	})

	// Wait for all goroutines in the errgroup to complete.
	return wg.Wait()
}
