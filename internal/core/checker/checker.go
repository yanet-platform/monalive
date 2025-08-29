// Package checker implements a health checking mechanism that handles the
// execution and processing of health checks, managing state transitions and
// event generation based on the results of these checks.
package checker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/yanet-platform/monalive/internal/core/checker/check"
	"github.com/yanet-platform/monalive/internal/scheduler"
	"github.com/yanet-platform/monalive/internal/types/weight"
	"github.com/yanet-platform/monalive/internal/types/xevent"
	"github.com/yanet-platform/monalive/internal/utils/shutdown"
	"github.com/yanet-platform/monalive/internal/utils/xnet"
)

// Check interface defines the behavior required for a health check
// implementation.
type Check interface {
	Do(ctx context.Context, md *check.Metadata) error
}

// ErrShutdown is an error that indicates a shutdown event has occurred.
var ErrShutdown = errors.New("shutdown")

// Checker handles the execution of health checks based on the provided
// configuration.
type Checker struct {
	config *Config
	check  Check // implementation of a health check

	state   State        // current state of the checker
	stateMu sync.RWMutex // to protect concurent access to the state

	handler  xevent.Handler // callback event handler function provided by the parent real
	eventsWG sync.WaitGroup // to manage goroutines handling events

	shutdown *shutdown.Shutdown
	log      *slog.Logger
}

// State represents the current state of the Checker.
type State struct {
	Weight         weight.Weight
	Alive          bool
	FailedAttempts int
	Timestamp      time.Time
}

// New creates a new Checker instance.
func New(config *Config, handler xevent.Handler, weight weight.Weight, forwardingData xnet.ForwardingData, logger *slog.Logger) *Checker {
	checker := &Checker{
		config:   config,
		handler:  handler,
		shutdown: shutdown.New(),
	}

	checker.state.Weight = weight

	// These values are used in the logger.
	var uri, meta string

	// Determine the type of check to use based on the configuration.
	switch config.Type {
	case TCPChecker:
		check := check.NewTCPCheck(config.CheckConfig, forwardingData)
		checker.check = check

		uri, meta = check.URI(), "tcp_check"

	case HTTPChecker:
		check := check.NewHTTPCheck(config.CheckConfig, forwardingData)
		checker.check = check

		uri, meta = check.URI(), "http_check"

	case HTTPSChecker:
		check := check.NewHTTPCheck(config.CheckConfig, forwardingData, check.HTTPWithTLS())
		checker.check = check

		uri, meta = check.URI(), "https_check"

	case GRPCChecker:
		check := check.NewGRPCCheck(config.CheckConfig, forwardingData)
		checker.check = check

		uri, meta = check.URI(), "grpc_check"
	}

	// Enhance the logger with context-specific information like URI and meta
	// information.
	checker.log = logger.With(
		slog.String("uri", uri),
		slog.Int("fwmark", config.Net.FWMark),
		slog.String("meta", meta),
	)

	return checker
}

// Run starts the checker loop, which periodically performs the health check. It
// first waits for a random delay to avoid all checkers running simultaneously.
// It then continues running the checks at intervals defined by the
// configuration until stopped.
func (m *Checker) Run(ctx context.Context) {
	scheduler := scheduler.New(m.config.Scheduler, scheduler.WithInitialDelay())
	m.log.Info(
		"running checker",
		slog.Duration("delay", scheduler.InitialDelay()),
		slog.String("event_type", "checker update"),
	)
	defer m.log.Info("checker stopped", slog.String("event_type", "checker update"))

	// Preventively increment the event counter so that checker's Stop function
	// doesn't complete until the shutdown event is handled properly.
	m.eventsWG.Add(1)

	// Generate the shutdown event when the function returns.
	defer func() {
		defer m.eventsWG.Done()

		md := check.Metadata{}
		md.SetInactive()
		m.ProcessCheck(md, ErrShutdown)
	}()

	// Create a child context for this checker that can be canceled
	// independently of the parent context.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// A goroutine is launched to handle shutdown signals.
	go func() {
		select {
		case <-ctx.Done():
			// Stop the checker if the parent context is canceled.
			m.Stop()
		case <-m.shutdown.Done():
			// Cancel the child context if a shutdown is signaled.
			cancel()
		}
	}()

	// Continuously execute checks at intervals specified by the configuration.
	_ = scheduler.Run(ctx, func() error {
		currState := m.State()
		md := check.Metadata{
			Alive:  currState.Alive,
			Weight: currState.Weight,
		}

		// Perform the check operation.
		opErr := m.check.Do(ctx, &md)

		// Process the result of the check operation.
		m.ProcessCheck(md, opErr)

		return opErr
	})
}

// Stop stops the checker gracefully, waiting for any ongoing checks to
// complete.
func (m *Checker) Stop() {
	// Trigger the shutdown signal to gracefully stop worker.
	m.shutdown.Do()
	// Wait for all checks to finish.
	m.eventsWG.Wait()
}

// State returns the current state of the checker.
func (m *Checker) State() State {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()

	return m.state
}
