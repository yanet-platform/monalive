// Package service implements the management of virtual server entity. It starts
// it's real server routins and handles events from them. Also, this layer
// performs interaction with announcer and balancer clients.
package service

import (
	"context"
	"sync"
	"time"

	log "go.uber.org/zap"

	"github.com/yanet-platform/monalive/internal/announcer"
	"github.com/yanet-platform/monalive/internal/balancer"
	"github.com/yanet-platform/monalive/internal/core/real"
	"github.com/yanet-platform/monalive/internal/types/key"
	"github.com/yanet-platform/monalive/internal/utils/shutdown"
	"github.com/yanet-platform/monalive/internal/utils/workerpool"
	"github.com/yanet-platform/monalive/pkg/weight"
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

	eventsWG     sync.WaitGroup // to manage goroutines handling events
	eventsMu     sync.Mutex
	eventsClosed bool

	lifeMu   sync.Mutex
	started  bool
	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once

	metrics *Metrics

	shutdown *shutdown.Shutdown
	log      *log.Logger
}

// State represents the current state of the service.
type State struct {
	Alive               bool
	Weight              weight.Weight
	RealsAlive          int
	Transitions         int
	TransitionTimestamp time.Time
}

// New creates a new Service instance.
func New(config *Config, announcer *announcer.Announcer, balancer *balancer.Balancer, logger *log.Logger) *Service {
	logger = logger.With(
		log.String("virtual_ip", config.VIP.String()),
		log.String("port", config.VPort.String()),
		log.String("protocol", config.Protocol),
	)
	defer logger.Info("service created", log.String("event_type", "service update"))

	return &Service{
		config: config,
		key:    config.Key(),

		announcer: announcer,
		balancer:  balancer,

		reals:     make(map[key.Real]*real.Real),
		realsPool: workerpool.New(),

		metrics: NewMetrics(),
		state:   State{TransitionTimestamp: time.Now()},

		shutdown: shutdown.New(),
		done:     make(chan struct{}),
		log:      logger,
	}
}

// Run starts the service.
//
// It runs the service's worker pool and performs a reload to apply the
// initial configuration for the reals. Cancellation or Stop joins all reals and
// their final callbacks before Run returns.
func (m *Service) Run(ctx context.Context) {
	m.lifeMu.Lock()
	select {
	case <-m.shutdown.Done():
		m.lifeMu.Unlock()
		<-m.done
		return
	default:
	}
	if m.started {
		m.lifeMu.Unlock()
		<-m.done
		return
	}

	m.started = true
	ctx, m.cancel = context.WithCancel(ctx)
	m.lifeMu.Unlock()
	defer func() {
		m.stopOnce.Do(m.finish)
		close(m.done)
	}()

	m.log.Info("running service", log.String("event_type", "service update"))
	defer m.log.Info("service stopped", log.String("event_type", "service update"))

	// Blocks access to the metrics after launching service.
	m.metrics.Block()

	// Staged Add permits installation before the pool starts. Serialize the
	// initial config read with Reload, rather than launching detached reloads.
	m.realsMu.Lock()
	m.reload(m.config)
	m.realsMu.Unlock()

	poolDone := make(chan struct{})
	go func() {
		m.realsPool.Run(ctx)
		close(poolDone)
	}()

	<-ctx.Done()
	m.stopOnce.Do(m.finish)
	<-poolDone
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

	m.reload(config)
}

func (m *Service) reload(config *Config) {
	select {
	case <-m.shutdown.Done():
		return
	default:
	}

	// Convert the keepalived reals configuration to the internal
	// representation.
	reals := make([]*real.Config, len(config.Reals))
	for i, r := range config.Reals {
		reals[i] = (*real.Config)(r)
	}

	// Prepare a new map to hold the new set of reals.
	// This map will eventually replace the existing reals map.
	newReals := make(map[key.Real]*real.Real)
	for _, cfg := range reals {
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
				newReal.SetMetrics(
					real.SetRealErrorsMetric(m.metrics.RealsErrors()),
					real.SetRealTransitionPeriodMetric(m.metrics.RealsTransitionPeriod()),
					real.SetRealResponseTimeMetric(m.metrics.RealsResponseTime()),
				)
				// Add the new real to the new reals map.
				newReals[key] = newReal
				// Add new real to the pool.
				m.realsPool.Add(newReal)
				// Increment the total number of reals as a new real was
				// created.
				m.metrics.RealsTotal().Add(1)
			}
		}
	}

	// After processing all new and existing reals, any remaining reals in the
	// old reals map are no longer valid and need to be stopped.
	for _, real := range m.reals {
		// Gracefully stop each outdated real.
		real.Stop()
		// Decrement the total number of reals.
		m.metrics.RealsTotal().Sub(1)
	}

	// Finally, replace the old reals map with the new one that contains the
	// updated set of reals and update service config.
	m.reals = newReals
	m.stateMu.Lock()
	forceAnnounce := m.config.AnnounceGroup == "" && config.AnnounceGroup != ""
	m.config = config

	// It is neccesary to process the status of the service announce after
	// reload due to possible changes in the announce settings.
	m.processAnnounce(forceAnnounce)
	m.stateMu.Unlock()
}

// Stop gracefully shuts down the service, stopping all associated reals and
// waiting for any ongoing event handling to complete before returning.
func (m *Service) Stop() {
	m.lifeMu.Lock()
	m.shutdown.Do()
	if m.cancel != nil {
		m.cancel()
	}
	started := m.started
	m.lifeMu.Unlock()

	if !started {
		m.stopOnce.Do(func() {
			m.finish()
			close(m.done)
		})
	}

	<-m.done
}

func (m *Service) finish() {
	m.lifeMu.Lock()
	m.shutdown.Do()
	if m.cancel != nil {
		m.cancel()
	}
	m.lifeMu.Unlock()

	// Lock the reals mutex to ensure thread-safe access.
	m.realsMu.Lock()
	defer m.realsMu.Unlock()
	// Gracefully stop all reals.
	for _, real := range m.reals {
		real.Stop()
	}

	m.realsPool.Close()

	// Children (including final shutdown callbacks) are joined before handler
	// admission closes. No Wait runs under a lock needed by those callbacks.
	m.eventsMu.Lock()
	m.eventsClosed = true
	m.eventsMu.Unlock()

	m.eventsWG.Wait()
}

// State returns a snapshot of the current service state.
func (m *Service) State() State {
	// Acquire a read lock on the state.
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.state
}

func (m *Service) SetMetrics(setMetricFunc ...SetMetricFunc) {
	for _, f := range setMetricFunc {
		f(m.metrics)
	}
}

func (m *Service) Key() key.Service {
	return m.key
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

		// LookupSubscription owns a helper that closes the channel on exit.
		// Drain through closure on every path so activation returns only after
		// that session-bound helper is joined.
		defer func() {
			for range subscription {
			}
		}()

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
