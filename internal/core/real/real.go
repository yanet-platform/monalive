// Package real implements the management of a real server entity.
package real

import (
	"context"
	"log/slog"
	"slices"
	"sync"

	"monalive/internal/core/checker"
	"monalive/internal/types/key"
	"monalive/internal/types/weight"
	"monalive/internal/types/xevent"
	"monalive/internal/utils/shutdown"
	"monalive/internal/utils/workerpool"
	"monalive/internal/utils/xnet"
)

// ActivationFunc defines a function type used to control the activation of a
// Real instance.
//
// It takes a context used to interrupt its execution and returns a boolean
// indicating whether the Real instance should be activated or not. This
// function allows for custom logic to be executed before the Real instance
// begins its operations, such as waiting for external conditions to be met.
type ActivationFunc func(ctx context.Context) (activated bool)

// Real represents a real server that performs health checks and maintains its
// operational state based on the results of those checks.
type Real struct {
	config *Config
	key    key.Real // used in events to set one-to-one correspondence to the current real

	checkers     map[checker.Key]*checker.Checker // current checkers mapped by their unique [checker.Key]
	checkersMu   sync.Mutex                       // to protect concurent access to the checkers map
	checkersPool *workerpool.Pool

	state   State        // current state of the real
	stateMu sync.RWMutex // to protect concurent access to the state

	handler  xevent.Handler // callback event handler function provided by the parent service
	eventsWG sync.WaitGroup // to manage goroutines handling events

	activationFunc ActivationFunc // used to control the activation of the real if necessary

	shutdown *shutdown.Shutdown
	log      *slog.Logger
}

// State represents the current state of the real.
type State struct {
	Alive       bool
	Weight      weight.Weight
	Transitions int

	// Whether dynamic weight calculation is supported by the real.
	DynWeight bool

	// If checker reports a failure and the inhibit_on_failure option is set in
	// the real's config, then instead of disabling it, we keep it enabled, but
	// set its weight to zero.
	//
	// NOTE: in fact, we do not change the weight of the real in its state,
	// because if the checker reports a successful check in the future, we must
	// enable the real with its weight before inhibition.
	Inhibited bool
}

// Status returns the current status of the real based on its state.
// If the service is inhibited, the status will reflect a weight of zero.
func (m State) Status() xevent.Status {
	if m.Inhibited {
		return xevent.Status{
			Enable: true,
			Weight: 0,
		}
	}
	return xevent.Status{
		Enable: m.Alive,
		Weight: m.Weight,
	}
}

// Option represents a function that configures a Real instance.
type Option func(*Real)

// WithActivationFunc returns an Option that sets the activation function for a
// Real instance. The activation function is used to determine whether the Real
// instance should be activated based on custom logic provided by the caller.
func WithActivationFunc(activation ActivationFunc) Option {
	return func(m *Real) {
		m.activationFunc = activation
	}
}

// New creates a new Real instance.
func New(config *Config, handler xevent.Handler, logger *slog.Logger, opts ...Option) *Real {
	logger = logger.With(
		slog.String("virtual_ip", config.IP.String()),
		slog.String("port", config.Port.String()),
	)
	defer logger.Info("real created", slog.String("event_type", "real update"))

	real := &Real{
		config:       config,
		key:          config.Key(),
		checkers:     make(map[checker.Key]*checker.Checker),
		checkersPool: workerpool.New(),
		handler:      handler,
		shutdown:     shutdown.New(),
		log:          logger,
	}

	// Apply optional configurations.
	for _, opt := range opts {
		opt(real)
	}

	return real
}

// Run starts the real.
//
// It runs the real's worker pool, waiting the real activation before launching
// it. The real will continue running until its Stop method is invoked.
func (m *Real) Run(ctx context.Context) {
	// Wait for the real activation before launching its checkers.
	if activated := m.waitActivation(); !activated {
		// If the activation function signals a non-activated status, return
		// early.
		return
	}

	m.log.Info("running real", slog.String("event_type", "real update"))
	defer m.log.Info("real stopped", slog.String("event_type", "real update"))

	// Reload to apply initial checkers configuration.
	go m.Reload(m.config)

	// Run the real's worker pool.
	m.checkersPool.Run(ctx)
}

// Reload updates the configuration of the real service, stopping outdated
// checkers and creating new ones as necessary.
func (m *Real) Reload(config *Config) {
	m.checkersMu.Lock()
	defer m.checkersMu.Unlock()

	// Prepare forwarding data that will be used by the checkers.
	forwardingData := xnet.ForwardingData{
		RealIP:           config.IP,
		ForwardingMethod: config.ForwardingMethod,
	}

	// Track if any checker supports dynamic weight.
	dynamicWeight := false

	// Concatenate all types of checkers from the new config into a single
	// slice.
	checkers := slices.Concat(
		config.TCPCheckers,
		config.HTTPCheckers,
		config.HTTPSCheckers,
		config.GRPCCheckers,
	)

	// Map to store the new set of checkers after reloading.
	newCheckers := make(map[checker.Key]*checker.Checker)
	for _, cfg := range checkers {
		select {
		// Check if the service is shutting down. If so, abort the reload.
		case <-m.shutdown.Done():
			m.log.Warn("reload aborted")
			return

		default:
			key := cfg.Key()
			switch knownChecker, exists := m.checkers[key]; exists {
			// If a checker with the same key already exists, reuse it.
			case true:
				newCheckers[key] = knownChecker
				// Remove from the old checkers map.
				delete(m.checkers, key)

			// If it's a new checker, create and initialize it.
			case false:
				newChecker := checker.New(
					cfg,
					m.HandleEvent,
					config.Weight,
					forwardingData,
					m.log,
				)
				newCheckers[key] = newChecker
				// Add new checker to the pool.
				m.checkersPool.Add(newChecker)
			}

			// Update the dynamicWeight flag.
			dynamicWeight = dynamicWeight || cfg.DynamicWeight
		}
	}

	// Stop any remaining old checkers that were not reused.
	for _, checker := range m.checkers {
		// Gracefully stop each outdated checker.
		checker.Stop()
	}

	// Update the state to reflect if dynamic weight is used.
	m.state.DynWeight = dynamicWeight

	// Save the new config and the updated set of checkers.
	m.config = config
	m.checkers = newCheckers
}

// Stop gracefully stops the real service, ensuring all checkers and event
// handling are properly terminated.
func (m *Real) Stop() {
	// Trigger the shutdown signal to gracefully stop workers.
	m.shutdown.Do()

	// Lock the checkers mutex to ensure thread-safe access.
	m.checkersMu.Lock()
	defer m.checkersMu.Unlock()
	// Gracefully stop all checkers.
	for _, checker := range m.checkers {
		checker.Stop()
	}

	// Wait for all event handling goroutines to finish.
	m.eventsWG.Wait()

	// Close the worker pool.
	m.checkersPool.Close()
}

// Key returns the unique key associated with the real.
func (m *Real) Key() key.Real {
	return m.key
}

// State returns the current state of the real.
func (m *Real) State() State {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.state
}

// waitActivation blocks execution until the activation function returns, if it
// has been set.
func (m *Real) waitActivation() (activated bool) {
	if m.activationFunc == nil {
		// If the activation function is not set, return early with an activated
		// status.
		return true
	}

	// Create a context with a cancel function to allow activationFunc to
	// properly stop its execution if the real shutdown occurs early.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start a goroutine to listen for either context cancellation or shutdown
	// signal.
	go func() {
		select {
		case <-ctx.Done():
			// Simply return because the activation function is expected to be
			// canceled when the provided context is canceled.
			return
		case <-m.shutdown.Done():
			// Cancel the activationFunc context if a shutdown is triggered to
			// avoid resource leaks.
			cancel()
		}
	}()

	// Call the activation function with the provided context.
	return m.activationFunc(ctx)
}
