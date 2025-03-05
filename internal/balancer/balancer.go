// Package balancer implements basic logic to manage the load balancer state.
//
// The package defines a Balancer type that uses a balancer Client interface to
// enable or disable real servers and a Stater interface to synchronize its
// internal state with the remote client. It handles events using an event
// registry and performs periodic updates and state synchronization.
package balancer

import (
	"context"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"monalive/internal/types/key"
	"monalive/internal/types/weight"
	"monalive/internal/types/xevent"
	event "monalive/internal/utils/event_registry"
	"monalive/internal/utils/shutdown"
)

// LoadBalancerClient defines the interface that a balancer client must implement.
// It provides methods to enable or disable a real server and to flush changes.
type LoadBalancerClient interface {
	EnableReal(ctx context.Context, key key.Balancer, weight weight.Weight) error
	DisableReal(ctx context.Context, key key.Balancer) error
	Flush(ctx context.Context) error
}

// Stater defines an interface for obtaining the current state of the load
// balancer reals.
type Stater interface {
	GetState(ctx context.Context) (map[string]services, error)
}

// Balancer manages the state of the load balancer by enabling or disabling real
// servers and synchronizing the state with a remote client.
type Balancer struct {
	config   *Config
	client   LoadBalancerClient                           // to communicate with the balancer
	state    *State                                       // keeps balancer state
	events   *event.Registry[key.Balancer, *xevent.Event] // stores events to the balancer
	shutdown *shutdown.Shutdown                           // manages graceful shutdown
	log      *slog.Logger
}

// New creates a new Balancer instance.
func New(config *Config, client LoadBalancerClient, logger *slog.Logger) *Balancer {
	var state *State
	// If the client implements the Stater interface, initialize the state.
	if _, implements := client.(Stater); implements {
		state = NewState()
	}

	return &Balancer{
		config:   config,
		client:   client,
		state:    state,
		events:   event.NewRegistry[key.Balancer, *xevent.Event](),
		shutdown: shutdown.New(),
		log:      logger,
	}
}

func (m *Balancer) SupportsState() (supports bool) {
	return m.state != nil
}

// Run starts the main processes, such as state synchronization and
// event handling.
func (m *Balancer) Run(ctx context.Context) error {
	// Removing cancellation to control shutdown through the internal mechanism.
	ctx = context.WithoutCancel(ctx)
	var wg errgroup.Group

	// Start the state synchronization process.
	wg.Go(func() error {
		return m.stater(ctx)
	})

	// Start the event updater process.
	wg.Go(func() error {
		return m.updater(ctx)
	})

	// Wait for all goroutines to finish.
	return wg.Wait()
}

// Stop triggers a graceful shutdown.
func (m *Balancer) Stop() {
	m.shutdown.Do()
}

// HandleEvent stores a new event into the event registry. Lately it will be
// passed to the load balancer client.
func (m *Balancer) HandleEvent(event *xevent.Event) {
	// Store the event associated with the specified balancer key.
	m.events.Store(event.Balancer, event)
}

// LookupSubscription returns a notification channel if a state change for the
// given key is detected. Subscription cancelation is performed by cancellation
// of provided context or balancer shutdown.
//
// It returns nil in case requested key already presented in the known balancer
// state or if balancer does not support state synchronization.
func (m *Balancer) LookupSubscription(ctx context.Context, key key.Balancer) <-chan struct{} {
	if m.state == nil {
		// If no state is managed, return nil.
		return nil
	}

	if found := m.state.Lookup(key); found {
		// If the key is already in the state, no need for a subscription.
		return nil
	}

	// Channel to notify subscribers. It is buffered to prevent blocks on
	// notifications.
	notify := make(chan struct{}, 1)

	// Launch a goroutine to wait for state updates or cancellation.
	go func() {
		defer close(notify)
		for {
			select {
			case <-ctx.Done():
				// If the context is canceled, exit the loop.
				return

			case <-m.shutdown.Done():
				// If shutdown is initiated, exit the loop.
				return

			// Wait for state updates.
			case <-m.state.Subscribe():
				if found := m.state.Lookup(key); found {
					// Notify the subscriber if the key is found.
					notify <- struct{}{}
					return
				}
			}
		}
	}()

	return notify
}

// stater periodically executes GetState request to the load balancer and stores
// the response state. If the load balancer does not support state
// synchronization, it returns nil.
func (m *Balancer) stater(ctx context.Context) error {
	client, implements := m.client.(Stater)
	if !implements {
		// If the client doesn't support state sync, just return.
		m.log.Warn("balancer client does not support state sync")
		return nil
	}

	// Initial state sync.
	state, err := client.GetState(ctx)
	if err != nil {
		m.log.Error("failed to get balancer state", slog.Any("error", err))
	}
	// Update the internal state with the fetched state.
	m.state.Update(state)

	// Timer for periodic state updates.
	updateTimer := time.NewTicker(m.config.SyncPeriod)
	defer updateTimer.Stop()

	for {
		select {
		case <-m.shutdown.Done():
			// Exit on shutdown.
			return nil

		// Trigger a state sync on each timer tick.
		case <-updateTimer.C:
			state, err := client.GetState(ctx)
			if err != nil {
				m.log.Error("failed to get balancer state", slog.Any("error", err))
				continue
			}
			// Update the internal state with the new state.
			m.state.Update(state)
		}
	}
}

// updater periodically flushes events and updates the balancer's configuration.
func (m *Balancer) updater(ctx context.Context) error {
	// Prevent cancellation to control shutdown through the internal mechanism.
	ctx = context.WithoutCancel(ctx)

	// Timer for periodic event updates.
	updateTicker := time.NewTicker(m.config.FlushPeriod)
	defer updateTicker.Stop()

	for {
		select {
		case <-m.shutdown.Done():
			// On shutdown, perform one last update and exit.
			m.updateBalancer(ctx)
			return nil

		// Trigger an update on each timer tick.
		case <-updateTicker.C:
			m.updateBalancer(ctx)
		}
	}
}

// updateBalancer processes pending events and applies them to the client.
func (m *Balancer) updateBalancer(ctx context.Context) {
	// Process each event in the registry.
	processed := m.events.Process(func(key key.Balancer, event *xevent.Event) error {
		var err error
		if event.New.Enable {
			// Enable the real server with the specified weight.
			err = m.client.EnableReal(ctx, key, event.New.Weight)
		} else {
			// Disable the real server.
			err = m.client.DisableReal(ctx, key)
		}
		if err != nil {
			m.log.Error(
				"could not update yanet real configuration",
				slog.Any("error", err),
				slog.String("event_type", "real handler"),
			)
		}
		return err
	})

	// If any events were processed, flush the changes to the client.
	if processed > 0 {
		if err := m.client.Flush(ctx); err != nil {
			m.log.Error("failed to flush balancer", slog.Any("error", err))
		}
	}
}
