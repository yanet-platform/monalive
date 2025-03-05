// Package eventregistry provides a generic registry for storing and processing
// events. Events can be merged, flushed, or processed using a custom processor
// function.
package eventregistry

import (
	"sync"
)

// Comparable defines an interface that must be implemented by types used in the
// Registry. The Merge method merges the current value with a new one and
// returns the result. The remove return value indicates whether the event
// should be removed from the registry.
type Comparable[V any] interface {
	Merge(value V) (result V, remove bool)
}

// Registry stores events associated with keys. It supports storing, merging,
// and processing events in a thread-safe manner.
type Registry[K comparable, V Comparable[V]] struct {
	events map[K]V
	mu     sync.Mutex
}

// NewRegistry creates and returns a new Registry instance.
func NewRegistry[K comparable, V Comparable[V]]() *Registry[K, V] {
	return &Registry[K, V]{
		events: make(map[K]V),
	}
}

// Store adds or merges an event in the registry associated with the given key.
// If the key already exists, the existing event is merged with the new one
// using the Merge method of the Comparable interface. If the result of the
// merge indicates that the event should be removed, it is deleted from the
// registry.
func (r *Registry[K, V]) Store(key K, value V) {
	r.mu.Lock()
	defer r.mu.Unlock()

	oldValue, exists := r.events[key]
	if !exists {
		// If the key does not exist, store the new value directly.
		r.events[key] = value
		return
	}

	// Merge the existing event with the new one.
	newValue, remove := oldValue.Merge(value)
	if remove {
		// If the merged result indicates removal, delete the event.
		delete(r.events, key)
		return
	}

	// Update the event with the merged result.
	r.events[key] = newValue
}

// Flush clears all events from the registry and returns them.
func (r *Registry[K, V]) Flush() map[K]V {
	r.mu.Lock()
	defer r.mu.Unlock()
	defer func() {
		r.events = make(map[K]V)
	}()

	return r.events
}

// Events returns a copy of the current events in the registry.
// Used for testing.
func (r *Registry[K, V]) Events() map[K]V {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.events
}

// Process applies the provided processor function to each event in the
// registry. Events for which the processor function returns no error are
// removed from the registry. The function returns the number of successfully
// processed events.
func (r *Registry[K, V]) Process(processor func(key K, event V) error) (processed int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, value := range r.events {
		if err := processor(key, value); err == nil {
			// If the processor function succeeds, increment the processed
			// counter and remove the event.
			processed++
			delete(r.events, key)
		}
	}
	return processed
}
