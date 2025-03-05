// Package shutdown provides a mechanism to signal a graceful shutdown process.
package shutdown

import (
	"sync"
)

// Shutdown manages a one-time shutdown signal.
type Shutdown struct {
	once   sync.Once     // ensures the shutdown signal is only sent once
	notify chan struct{} // used to notify listeners of shutdown
}

// New creates a new Shutdown instance.
func New() *Shutdown {
	return &Shutdown{
		notify: make(chan struct{}),
	}
}

// Do triggers the shutdown signal if it hasn't been triggered already.
// It closes the notify channel to signal all listeners.
func (m *Shutdown) Do() {
	m.once.Do(func() { close(m.notify) })
}

// Done returns a channel that is closed when the shutdown signal is triggered.
// This allows goroutines to listen for the shutdown signal.
func (m *Shutdown) Done() <-chan struct{} {
	return m.notify
}
