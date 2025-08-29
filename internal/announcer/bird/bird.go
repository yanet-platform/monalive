// Package bird provides an implementation of the [announcer.Client] interface for
// managing prefix announces using [xbird.Client] connector.
package bird

import (
	"errors"
	"fmt"
	"net"
	"net/netip"

	xbird "github.com/yanet-platform/monalive/pkg/bird"

	"github.com/yanet-platform/monalive/internal/announcer"
)

// Bird is an implementation of the announcer.Client interface that interacts
// with the BIRD routing daemon to manage route announces.
type Bird struct {
	clients map[string]*xbird.Client // maps group names to their corresponding BIRD clients
}

// New creates a new Bird instance that manages multiple BIRD clients for
// different groups. Returns an error if any client fails to initialize.
func New(config *Config, groups []string) (*Bird, error) {
	clients := make(map[string]*xbird.Client, len(groups))
	for _, group := range groups {
		client, err := xbird.NewClient(config.SockDir, group)
		if err != nil {
			return nil, fmt.Errorf("failed to create bird client for group %q: %w", group, err)
		}

		clients[group] = client
	}

	return &Bird{
		clients: clients,
	}, nil
}

// RaiseAnnounce enables the announce of a given prefix in the specified group.
func (m *Bird) RaiseAnnounce(group string, prefix netip.Prefix) error {
	return m.processAnnounce(group, prefix, true)
}

// RemoveAnnounce disables an announce for a given prefix in the specified
// group.
func (m *Bird) RemoveAnnounce(group string, prefix netip.Prefix) error {
	return m.processAnnounce(group, prefix, false)
}

// ProcessBatch processes a batch of prefix announces for a given group. It
// sends the prefixes with their corresponding status (ready or unready) to the
// BIRD daemon in a single operation.
func (m *Bird) ProcessBatch(group string, announces map[netip.Prefix]announcer.PrefixStatus) error {
	client, err := m.clientByGroup(group)
	if err != nil {
		return err
	}

	// Prepare messages to be sent in a batch to the BIRD daemon.
	msgs := make([]xbird.Message, 0, len(announces))
	for prefix, status := range announces {
		birdStatus := xbird.Disable
		if status == announcer.Ready {
			birdStatus = xbird.Enable
		}

		// Create a message for each prefix and its status.
		msgs = append(msgs, xbird.NewMessage(prefix, birdStatus))
	}

	// Send all messages in a batch to the BIRD daemon.
	if err := client.SendBatch(msgs...); err != nil {
		return fmt.Errorf("failed to send batch of messages to bird: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down all BIRD clients managed by this instance. It
// closes all active Unix domain sockets used for communication.
func (m *Bird) Shutdown() {
	for _, client := range m.clients {
		client.Shutdown()
	}
}

// ListenStateRequest listens for state requests from BIRD for the given group.
// This function blocks until a request is received or an error occurs.
func (m *Bird) ListenStateRequest(group string) error {
	client, err := m.clientByGroup(group)
	if err != nil {
		return err
	}

	// Listen for an incoming state request from the BIRD daemon.
	err = client.ListenRequest()
	if errors.Is(err, net.ErrClosed) {
		// [net.ErrClosed] means that the client shutdown has been performed,
		// therefore, return [announcer.ErrShutdown] here as required by
		// [announcer.Stater].
		return announcer.ErrShutdown
	}
	return err
}

// processAnnounce processes a single prefix announce for a given group. It
// determines whether to enable or disable the announce based on the 'enable'
// flag.
func (m *Bird) processAnnounce(group string, prefix netip.Prefix, enable bool) error {
	client, err := m.clientByGroup(group)
	if err != nil {
		return err
	}

	// Determine the status to send to the BIRD daemon (enable or disable).
	status := xbird.Disable
	if enable {
		status = xbird.Enable
	}

	// Create a message and send it to the BIRD daemon.
	msg := xbird.NewMessage(prefix, status)
	if err := client.Send(msg); err != nil {
		return fmt.Errorf("failed to send message to bird: %w", err)
	}

	return nil
}

// clientByGroup returns the BIRD client for the specified group.
// Returns an error if the client is not configured.
func (m *Bird) clientByGroup(group string) (*xbird.Client, error) {
	client := m.clients[group]
	if client == nil {
		return nil, fmt.Errorf("bird client for group %q is not configured", group)
	}

	return client, nil
}
