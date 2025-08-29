package balancer

import (
	"sync"

	"github.com/yanet-platform/monalive/internal/types/key"
)

type (
	services = map[key.Service]reals
	reals    = map[key.Real]struct{}
)

// State manages the load balancer state and notifies subscribers on any
// updates.
type State struct {
	state   map[string]services // current state of the load ballancer mapped by its module key
	stateMu sync.RWMutex        // for synchronizing access to the state map

	notify   chan struct{} // channel to notify subscribers of state changes
	notifyMu sync.RWMutex  // for synchronizing access to the notification channel
}

// NewState creates a new State instance.
func NewState() *State {
	return &State{
		state:  map[string]services{},
		notify: make(chan struct{}),
	}
}

// Update updates the known load balancer state with the provided one and
// notifies subscribers.
func (m *State) Update(state map[string]services) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	// Update the internal state with the new state provided.
	m.state = state
	// Trigger notification to subscribers about the state change.
	m.notification()
}

// Lookup checks if a given balancer key exists in the current state.
func (m *State) Lookup(key key.Balancer) (found bool) {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()

	// Iterate over the services in the state to check if the provided key
	// exists.
	for _, services := range m.state {
		if reals, ok := services[key.Service]; ok {
			if _, ok := reals[key.Real]; ok {
				return true
			}
		}
	}
	return false
}

// Subscribe returns a channel that is closed when the state is updated. This
// allows subscribers to be notified of state changes.
func (m *State) Subscribe() <-chan struct{} {
	m.notifyMu.RLock()
	defer m.notifyMu.RUnlock()
	// Return the current notification channel.
	return m.notify
}

// notification triggers a state change notification by closing the existing
// channel and creating a new one for future notifications.
func (m *State) notification() {
	m.notifyMu.Lock()
	defer m.notifyMu.Unlock()

	// Close the existing notification channel to notify subscribers.
	close(m.notify)
	// Create a new notification channel for future notifications.
	m.notify = make(chan struct{})
}
