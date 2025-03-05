// Package scheduler provides functionality for scheduling and executing jobs
// with configurable delays, retry mechanisms, and optional initial random
// delay.
package scheduler

import (
	"context"
	"math/rand/v2"
	"time"
)

// Scheduler defines the structure to hold the configuration and initial delay
// for running jobs.
type Scheduler struct {
	config    Config        // holds the scheduling configuration
	initDelay time.Duration // initial random delay before starting the scheduler
}

// Option represents a function that configures a Scheduler instance.
type Option func(*Scheduler)

// WithInitialDelay returns an Option that applies an initial random delay to
// the Scheduler before running the job. The delay is a random value between 0
// and the configured loop delay.
func WithInitialDelay() Option {
	return func(s *Scheduler) {
		s.initDelay = time.Duration(rand.Float64() * float64(s.config.GetDelayLoop()))
	}
}

// New creates a new Scheduler instance.
func New(config Config, opts ...Option) *Scheduler {
	s := &Scheduler{
		config: config,
	}

	// Apply all provided options to the Scheduler.
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Run starts the scheduler and continuously runs the job according to the
// configured delays. It stops when the context is canceled. The scheduler
// includes retries in case of job failure, followed by a configured retry
// delay.
func (m *Scheduler) Run(ctx context.Context, job func() error) error {
	// Start with the initial delay.
	//
	// NOTE: we intentionally create a single timer for both retries and the
	// main execution loop. Since the scheduler cannot be in a state of waiting
	// for a retry and a loop delay simultaneously, reusing the same timer
	// simplifies managing the timeouts. Additionally, it ensures that the
	// [time.Timer.Reset] method is called on an already expired timer, avoiding
	// potential race conditions that can occur if Reset is invoked on a running
	// timer.
	timer := time.NewTimer(m.initDelay)
	defer timer.Stop()

	// Check if the context is already canceled to avoid a situation where the
	// timer has already expired and the context is also canceled. In such a
	// case, the select statement bellow could randomly select between the
	// context cancellation and the timer expiration, potentially leading to
	// unexpected behavior.
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Wait for the initial delay.
	select {
	case <-ctx.Done():
		// Stop if the context is canceled.
		return ctx.Err()
	case <-timer.C:
	}

	retries := m.config.GetRetries()
	retryDelay := m.config.GetRetryDelay()
	loopDelay := m.config.GetDelayLoop()

	for {
		// Attempt to run the job and retry upon failure.
		for i := 0; i <= retries; i++ {
			if err := job(); err == nil {
				// Job succeeded, break out of retry loop.
				break
			}

			if i == retries {
				// Maximum retries reached, break.
				break
			}

			// Retry after the configured delay.
			timer.Reset(retryDelay)
			select {
			case <-ctx.Done():
				// Stop if the context is canceled.
				return ctx.Err()
			case <-timer.C:
			}
		}

		timer.Reset(loopDelay)
		// Wait for the loop delay before starting the next job execution.
		select {
		case <-ctx.Done():
			// Stop if the context is canceled.
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// InitialDelay returns the initial random delay applied to the scheduler.
func (m *Scheduler) InitialDelay() time.Duration {
	return m.initDelay
}
