// Package service implements the management of virtual server entity. It starts
// it's real server routins and handles events from them. Also, this layer
// performs interaction with announcer and balancer clients.
package service

import (
	"context"
	"log/slog"
	"sync"

	"github.com/yanet-platform/monalive/internal/announcer"
	"github.com/yanet-platform/monalive/internal/balancer"
	"github.com/yanet-platform/monalive/internal/core/real"
	"github.com/yanet-platform/monalive/internal/types/key"
	"github.com/yanet-platform/monalive/internal/types/weight"
	"github.com/yanet-platform/monalive/internal/utils/shutdown"
	"github.com/yanet-platform/monalive/internal/utils/workerpool"
)

// Service represents a virtual server with underlying reals (individual backend
// instances). It manages the lifecycle and state of the service, including
// reals and events from them.
type Service struct {
	config *Config
	key    key.Service // used in events to set one-to-one correspondence to the current service

	announcer *announcer.Announcer // to update annouce status (enable/disable) of current service
	balancer  *balancer.Balancer   // updates real servers state in the load balancer according health check results

	reals     map[key.Real]*real.Real // current reals mapped by their unique [key.Real]
	realsMu   sync.Mutex              // to protect concurent access to the reals map
	realsPool *workerpool.Pool

	state   State        // current state of the service
	stateMu sync.RWMutex // to protect concurent access to the state

	eventsWG sync.WaitGroup // to manage goroutines handling events

	shutdown *shutdown.Shutdown
	log      *slog.Logger
}

// State represents the current state of the service.
type State struct {
	Alive       bool
	Weight      weight.Weight
	RealsAlive  int
	Transitions int
}

// New creates a new Service instance.
func New(config *Config, announcer *announcer.Announcer, balancer *balancer.Balancer, logger *slog.Logger) *Service {
	logger = logger.With(
		slog.String("virtual_ip", config.VIP.String()),
		slog.String("port", config.VPort.String()),
		slog.String("protocol", config.Protocol),
	)
	defer logger.Info("service created", slog.String("event_type", "service update"))

	return &Service{
		config: config,
		key:    config.Key(),

		announcer: announcer,
		balancer:  balancer,

		reals:     make(map[key.Real]*real.Real),
		realsPool: workerpool.New(),

		shutdown: shutdown.New(),
		log:      logger,
	}
}

// Run starts the service.
//
// It runs the service's worker pool and performs a reload to apply the
// initial configuration for the reals. The service will continue running until
// its Stop method is invoked.
func (m *Service) Run(ctx context.Context) {
	m.log.Info("running service", slog.String("event_type", "service update"))
	defer m.log.Info("service stopped", slog.String("event_type", "service update"))

	// Reload to apply initial reals configuration.
	go m.Reload(m.config)

	// Run the service's worker pool.
	m.realsPool.Run(ctx)
}

// Reload updates the service with a new configuration.
//
// It either updates existing reals or adds new ones based on the new
// configuration. After, it stops reals that are no longer present in the new
// configuration.
func (m *Service) Reload(config *Config) {
	// Lock the reals mutex to ensure thread-safe access.
	m.realsMu.Lock()
	defer m.realsMu.Unlock()

	// Prepare a new map to hold the new set of reals.
	// This map will eventually replace the existing reals map.
	newReals := make(map[key.Real]*real.Real)
	for _, cfg := range config.Reals {
		select {
		// Check if the shutdown signal has been triggered.
		case <-m.shutdown.Done():
			m.log.Warn("reload aborted")
			return

		default:
			// Extract the unique key for the current real configuration.
			key := cfg.Key()
			switch knownReal, exists := m.reals[key]; exists {
			case true:
				// If the real already exists in the current reals map, it means
				// we're updating an existing real with new configuration
				// details. Invokes the Reload method on the real to apply new
				// checkers configuration of this real.
				knownReal.Reload(cfg)
				// Add this updated real to the new reals map.
				newReals[key] = knownReal
				// Remove the real from the old reals map as it has been
				// processed.
				delete(m.reals, key)

			case false:
				// If the real doesn't exist in the current reals map, it's a
				// new real. Create a new real instance with the provided
				// configuration.
				var realOpts []real.Option
				if m.balancer.SupportsState() {
					realOpts = append(
						realOpts,
						real.WithActivationFunc(m.realActivationFunc(key)),
					)
				}
				newReal := real.New(cfg, m.HandleEvent, m.log, realOpts...)

				// Add the new real to the new reals map.
				newReals[key] = newReal
				// Add new real to the pool.
				m.realsPool.Add(newReal)
			}
		}
	}

	// After processing all new and existing reals, any remaining reals in the
	// old reals map are no longer valid and need to be stopped.
	for _, real := range m.reals {
		// Gracefully stop each outdated real.
		real.Stop()
	}

	// Finally, replace the old reals map with the new one that contains the
	// updated set of reals and update service config.
	m.reals = newReals
	m.config = config

	// It is neccesary to process the status of the service announce after
	// reload due to possible changes in the announce settings.
	m.processAnnounce()
}

// Stop gracefully shuts down the service, stopping all associated reals and
// waiting for any ongoing event handling to complete before returning.
func (m *Service) Stop() {
	// Trigger the shutdown signal to gracefully stop workers.
	m.shutdown.Do()

	// Lock the reals mutex to ensure thread-safe access.
	m.realsMu.Lock()
	defer m.realsMu.Unlock()
	// Gracefully stop all reals.
	for _, real := range m.reals {
		real.Stop()
	}

	// Wait for all event handling goroutines to finish.
	m.eventsWG.Wait()

	// Close the worker pool.
	m.realsPool.Close()
}

// State returns a snapshot of the current service state.
func (m *Service) State() State {
	// Acquire a read lock on the state.
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.state
}

// realActivationFunc returns an activation function for a given real.
func (m *Service) realActivationFunc(real key.Real) real.ActivationFunc {
	return func(ctx context.Context) (activated bool) {
		balancerKey := key.Balancer{Service: m.key, Real: real}
		subscription := m.balancer.LookupSubscription(ctx, balancerKey)

		if subscription == nil {
			// If the subscription channel is nil, assume the real is ready and
			// return true.
			return true
		}

		select {
		case <-ctx.Done():
			// If the context is canceled, return false to indicate activation
			// has failed due to cancellation.
			return false
		case _, found := <-subscription:
			// If an update is received from the subscription, return its status
			// based on whether the subscription channel was closed or not.
			return found
		}
	}
}
