// Package core implements the basic logic of health checking virtual and real
// servers.
package core

import (
	"context"
	"fmt"
	"sync"

	log "go.uber.org/zap"

	"github.com/yanet-platform/monalive/internal/announcer"
	"github.com/yanet-platform/monalive/internal/balancer"
	"github.com/yanet-platform/monalive/internal/core/service"
	"github.com/yanet-platform/monalive/internal/monitoring/metrics"
	"github.com/yanet-platform/monalive/internal/types/key"
	"github.com/yanet-platform/monalive/internal/utils/shutdown"
	"github.com/yanet-platform/monalive/internal/utils/workerpool"
)

// Core manages the lifecycle of the services.
type Core struct {
	announcer *announcer.Announcer // to reload announcer config to keep it in sync with known virtual servers
	balancer  *balancer.Balancer   // only to pass it to the new services

	services     map[key.Service]*service.Service // current services mapped by their unique [key.Service]
	servicesMu   sync.Mutex                       // to protect concurent access to the services map
	servicesPool *workerpool.Pool

	metrics *Metrics

	shutdown *shutdown.Shutdown
	log      *log.Logger
}

// New creates a new Core instance.
func New(announcer *announcer.Announcer, balancer *balancer.Balancer, metrics *metrics.ScopedMetrics, logger *log.Logger) *Core {
	return &Core{
		announcer: announcer,
		balancer:  balancer,

		services:     map[key.Service]*service.Service{},
		servicesPool: workerpool.New(),

		metrics: NewMetrics(metrics),

		shutdown: shutdown.New(),
		log:      logger,
	}
}

// Run starts the Core service.
//
// It runs the core's worker pool. The core will continue running until its Stop
// method is invoked.
func (m *Core) Run(ctx context.Context) {
	m.log.Info("running core")
	defer m.log.Info("core stopped")

	m.servicesPool.Run(ctx)
}

// Reload updates the Core with a new configuration.
//
// It first updates the announcer with the new services' announce groups,
// ensuring the announcer is in sync with the latest configuration. Then, it
// either updates existing services or adds new ones based on the new
// configuration. Finally, it stops services that are no longer present in the
// new configuration and replaces the old services with the new ones.
func (m *Core) Reload(config *Config) error {
	// Lock the services mutex to ensure thread-safe access.
	m.servicesMu.Lock()
	defer m.servicesMu.Unlock()

	// It is crutial to update the announcer first, as it will immediately
	// remove announces of the deleted services.
	//
	// Construct mapping of services to their announce groups.
	servicesForAnnouncer := make(map[key.Service]string)
	for _, cfg := range config.Services {
		if cfg.AnnounceGroup != "" {
			servicesForAnnouncer[cfg.Key()] = cfg.AnnounceGroup
		}
	}
	// Reload the announcer with new services.
	if err := m.announcer.ReloadServices(servicesForAnnouncer); err != nil {
		return fmt.Errorf("failed to reload announcer: %w", err)
	}

	// Prepare a new map to hold the new set of services.
	// This map will eventually replace the existing services map.
	newServices := make(map[key.Service]*service.Service)
	for _, cfg := range config.Services {
		select {
		// Check if the shutdown signal has been triggered.
		case <-m.shutdown.Done():
			m.log.Warn("core reload aborted")
			return nil

		default:
			// Extract the unique [key.Service] for the current service
			// configuration.
			key := cfg.Key()
			switch knownService, exists := m.services[key]; exists {
			case true:
				// If the service already exists in the current services map, it
				// means we're updating an existing service with new
				// configuration details. Invokes the Reload method on the
				// service to apply new reals and checkers configuration.
				knownService.Reload(cfg)
				// Add this updated service to the new services map.
				newServices[key] = knownService
				// Remove the service from the old services map as it has been
				// processed.
				delete(m.services, key)

			case false:
				// If the service doesn't exist in the current services map,
				// it's a new service. Create a new service instance with the
				// provided configuration.
				newService := service.New(cfg, m.announcer, m.balancer, m.log)
				serviceLabels := key.Labels()
				newService.SetMetrics(
					service.SetRealsEnabledMetric(m.metrics.RealsEnabledForService(serviceLabels)),
					service.SetRealsMetric(m.metrics.RealsForService(serviceLabels)),
					service.SetRealsTransitionPeriodMetric(m.metrics.RealsTrasitionPeriodForService(serviceLabels)),
					service.SetRealsResponseTimeMetric(m.metrics.RealsResponseTimeForService(serviceLabels)),
					service.SetRealsErrorsMetric(m.metrics.RealsErrorsForService(serviceLabels)),
				)

				// Add the new service to the new services map.
				newServices[key] = newService
				// Add new service to the pool.
				m.servicesPool.Add(newService)
			}
		}
	}

	// After processing all new and existing services, any remaining services in
	// the old services map are no longer valid and need to be stopped.
	for _, service := range m.services {
		// Gracefully stop each outdated service.
		service.Stop()
		m.metrics.DeleteService(service.Key().Labels())
	}

	// Finally, replace the old services map with the new one that contains the
	// updated set of services.
	m.services = newServices

	return nil
}

// Stop initiates a graceful shutdown of the Core and all its services.
// It ensures that no new services are started, and existing services are
// stopped. It uses the shutdown mechanism to signal that the Core should no
// longer accept new tasks and performs cleanup for all services.
func (m *Core) Stop() {
	// Trigger the shutdown signal to gracefully stop workers.
	m.shutdown.Do()

	// Lock the services mutex to ensure thread-safe access.
	m.servicesMu.Lock()
	defer m.servicesMu.Unlock()
	// Gracefully stop all services.
	for _, service := range m.services {
		service.Stop()
	}
	// Close the worker pool.
	m.servicesPool.Close()
}
