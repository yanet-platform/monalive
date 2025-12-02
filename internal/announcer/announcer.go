// Package announcer implements a layer for handling prefix announces between
// the Core part of monalive and external systems. The package performs two main
// tasks: it maintains the state of prefixes based on events sent from Core via
// the UpdateService method and provides necessary interactions with an external
// announcer instance through the announcer.Client interface.
package announcer

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/yanet-platform/monalive/internal/types/key"
	event "github.com/yanet-platform/monalive/internal/utils/event_registry"
	"github.com/yanet-platform/monalive/internal/utils/shutdown"
)

var (
	// ErrShutdown is returned when the announcer is shutdown.
	ErrShutdown = errors.New("announcer is shutdown")

	// ErrUnknownGroup is returned when the prefix group is not found.
	ErrUnknownGroup = errors.New("unknown group")
)

// Client is an interface that defines methods that external announcer client
// must implements. It includes methods for raising and removing announces,
// processing batches of prefix updates, and shutting down the client.
type Client interface {
	RaiseAnnounce(group string, prefix netip.Prefix) error
	RemoveAnnounce(group string, prefix netip.Prefix) error
	ProcessBatch(group string, prefixes map[netip.Prefix]PrefixStatus) error
	Shutdown()
}

// Stater is an interface that defines a method for listening to state requests.
// This is used to receive and respond to state requests from external announcer
// instance for a specific group.
//
// ListenStateRequest implementation must return [ErrShutdown] on client
// shutdown. Otherwise, the state listener worker won't be stopped properly.
type Stater interface {
	// ListenStateRequest listens for state requests from external announcer
	// instance for a specific group. Returns [ErrShutdown] on the client
	// shutdown.
	ListenStateRequest(group string) error
}

// Announcer is responsible for managing prefix announces across multiple
// groups. It maintains the configuration, announcer client, and internal state
// required to synchronize prefix statuses and handle updates.
type Announcer struct {
	config                *Config
	client                Client                                      // client to communicate with an external announcer instance
	prefixes              PrefixesGroups                              // groups of prefixes associated with their respective services
	serviceEventRegistry  *event.Registry[key.Service, ServiceStatus] // stores service status updates using the service as a key
	announceGroupByPrefix *AnnounceGroupByPrefix                      // contains association between prefix and it's announce group
	shutdown              *shutdown.Shutdown                          // shutdown mechanism to handle graceful termination
	log                   *slog.Logger
}

// New creates a new instance of Announcer.
func New(config *Config, client Client, logger *slog.Logger) *Announcer {
	return &Announcer{
		config:                config,
		client:                client,
		prefixes:              NewPrefixesGroups(config.AnnounceGroup),
		serviceEventRegistry:  event.NewRegistry[key.Service, ServiceStatus](),
		announceGroupByPrefix: NewAnnounceGroupByPrefix(),
		shutdown:              shutdown.New(),
		log:                   logger,
	}
}

// Run starts the main processes of the Announcer: updating external announcer
// state acording to the internal one. It also handles state requests from an
// external announcer, if neccesary.
//
// This function intentionally runs without using a context to ensure more
// controlled and explicit shutdown of the announcer via the Stop method.
func (m *Announcer) Run() error {
	var wg errgroup.Group

	// Start the state request handler.
	wg.Go(func() error {
		return m.stateRequestHandler()
	})

	// Start the updater process.
	wg.Go(func() error {
		m.updater()
		return nil
	})

	// Wait for both goroutines to complete.
	return wg.Wait()
}

// RegisterServiceEvent stores passed status of the service to the event
// registry.
func (m *Announcer) RegisterServiceEvent(service key.Service, status ServiceStatus) {
	m.serviceEventRegistry.Store(service, status)
}

// FlushServiceEvents flushes all service events from the event registry and
// returns them.
func (m *Announcer) FlushServiceEvents() map[key.Service]ServiceStatus {
	return m.serviceEventRegistry.Flush()
}

// UpdateService updates the status of a service.
//
// Changing the status of service may change it's host prefix announce depending
// on the status of other services with the same host prefix.
func (m *Announcer) UpdateService(service key.Service, status ServiceStatus) error {
	group, exists := m.announceGroupByPrefix.GetAnnounceGroup(service.Prefix())
	if !exists {
		return fmt.Errorf("failed to determine announce group for the prefix %q", service.Prefix())
	}
	prefixesGroup := m.prefixes.GetGroup(group)
	if prefixesGroup == nil {
		return fmt.Errorf("%w: %q", ErrUnknownGroup, group)
	}
	return prefixesGroup.UpdateService(service, status)
}

// ReloadServices reloads the list of services for each announce group. Its also
// updates current host prefix statuses according to the new services
// configuration.
func (m *Announcer) ReloadServices(servicesGroups map[string][]key.Service) error {
	// Validate announce groups before performing reload.
	for group := range servicesGroups {
		if prefixesGroup := m.prefixes.GetGroup(group); prefixesGroup == nil {
			return fmt.Errorf("%w: %q", ErrUnknownGroup, group)
		}
	}
	// Construct mapping of prefixes to their announce group.
	groupByPrefix := make(map[netip.Prefix]string)
	for group, services := range servicesGroups {
		for _, service := range services {
			prefix := service.Prefix()
			if knownGroup, exists := groupByPrefix[prefix]; exists && knownGroup != group {
				return fmt.Errorf("duplicate announce group prefix: %s", prefix)
			}
			groupByPrefix[prefix] = group
		}
	}
	// Update the groupByPrefix mapping.
	m.announceGroupByPrefix.Update(groupByPrefix)
	// Reload services for each group.
	for group, prefixes := range m.prefixes {
		// It's ok if we get zero services for a group here.
		services := servicesGroups[group]
		prefixes.ReloadServices(services)
	}
	return nil
}

// Stop gracefully stops the Announcer.
// It triggers the shutdown mechanism, removes all announces, and shuts down the
// announcer client.
func (m *Announcer) Stop() {
	// Signal shutdown to ongoing processes.
	m.shutdown.Do()
	// Remove all prefix announces.
	m.removeAll()
	// Shut down the announcer client.
	m.client.Shutdown()
}

// updater periodically sends new updated prefix statuses to an external
// announcer instance.
func (m *Announcer) updater() {
	var wg sync.WaitGroup

	// Set the wait group counter to the number of update workers.
	wg.Add(len(m.config.AnnounceGroup))

	// Launch an updater goroutine for each group.
	for _, group := range m.config.AnnounceGroup {
		go func() {
			defer wg.Done()
			m.groupUpdater(group)
		}()
	}

	// Wait for all updater goroutines to complete.
	wg.Wait()
}

// groupUpdater is a worker of [updater] routine that periodically sends updated
// prefix statuses for specified announce group to an external announcer
// instance.
func (m *Announcer) groupUpdater(group string) {
	prefixesGroup := m.prefixes.GetGroup(group)
	if prefixesGroup == nil {
		m.log.Error(
			"failed to update group",
			slog.String("group_name", group),
			slog.Any("error", ErrUnknownGroup),
		)
		return
	}

	updateTimer := time.NewTimer(m.config.UpdatePeriod)
	defer updateTimer.Stop()

	for {
		select {
		case <-m.shutdown.Done():
			// Exit if a shutdown signal is received.
			return

		case <-updateTimer.C:
			// Check and process any prefix status updates.
			events := prefixesGroup.Events()
			if len(events) == 0 {
				continue
			}

			// Send the update events to the client for processing.
			if err := m.client.ProcessBatch(group, events); err != nil {
				m.log.Error(
					"failed to sync announces state",
					slog.String("group_name", group),
					slog.Any("error", err),
				)
			}
		}
	}
}

// stateRequestHandler listens for state requests if the client supports it. It
// responds with the current status of prefixes for each announce group.
func (m *Announcer) stateRequestHandler() error {
	client, implements := m.client.(Stater)
	if !implements {
		// If the client doesn't implement the Stater interface, return early.
		return nil
	}

	var wg errgroup.Group
	// Listen for state requests for each group.
	for _, group := range m.config.AnnounceGroup {
		wg.Go(func() error {
			prefixesGroup := m.prefixes.GetGroup(group)
			if prefixesGroup == nil {
				m.log.Error(
					"failed to setup state request handler",
					slog.String("group_name", group),
					slog.Any("error", ErrUnknownGroup),
				)
				return nil
			}
			for {
				// Handle state requests for the group.
				if err := client.ListenStateRequest(group); err != nil {
					if errors.Is(err, ErrShutdown) {
						// Stop the worker on shutdown.
						return err
					}
					// Other errors must be logged, but does not terminate the
					// lifecycle of the worker.
					m.log.Error(
						"failed to handle state request",
						slog.String("group_name", group),
						slog.Any("error", err),
					)
					continue
				}

				// Respond with the current prefix statuses.
				prefixesStatus := prefixesGroup.Status()
				if err := m.client.ProcessBatch(group, prefixesStatus); err != nil {
					m.log.Error(
						"failed to sync announces state",
						slog.String("group_name", group),
						slog.Any("error", err),
					)
				}
			}
		})
	}

	// Wait for all state request handling goroutines to complete.
	return wg.Wait()
}

// removeAll removes all prefix announces for every known group.
// This is typically called during the shutdown process to ensure no announces
// remain active.
func (m *Announcer) removeAll() {
	for _, group := range m.config.AnnounceGroup {
		prefixes := m.prefixes.GetGroup(group)
		if prefixes == nil {
			continue
		}
		prefixesStatus := prefixes.Status()

		// Mark all prefixes as Unready to indicate they are being removed.
		for prefix := range prefixesStatus {
			prefixesStatus[prefix] = Unready
		}

		// Send the removal updates to the client.
		if err := m.client.ProcessBatch(group, prefixesStatus); err != nil {
			m.log.Error(
				"failed to remove announces",
				slog.String("group_name", group),
				slog.Any("error", err),
			)
		}
	}
}
