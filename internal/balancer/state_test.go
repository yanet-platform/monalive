package balancer

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"monalive/internal/types/key"
)

// TestState_Lookup tests the Lookup method of the State struct.
// It verifies that the Lookup method correctly identifies whether
// a key is present in the state map after updating the state.
func TestState_Lookup(t *testing.T) {
	state := NewState()
	// Define and initialize keys for services and reals.
	defaultKey := defaultKey()

	service1 := defaultKey.Service
	service1.Addr = netip.MustParseAddr("127.0.1.1")
	real1 := defaultKey.Real
	real1.Addr = netip.MustParseAddr("127.0.1.2")

	service2 := defaultKey.Service
	service2.Addr = netip.MustParseAddr("127.0.2.1")
	real2 := defaultKey.Real
	real2.Addr = netip.MustParseAddr("127.0.2.2")

	// Initially, the state should not contain any services.
	found := state.Lookup(key.Balancer{Service: service1, Real: real1})
	assert.False(t, found)

	// Update the state with new services and reals.
	update := map[string]services{
		"module1": {
			service1: {
				real1: {},
			},
			service2: {
				real2: {},
			},
		},
	}
	state.Update(update)

	// After updating the state, the key should be found.
	found = state.Lookup(key.Balancer{Service: service1, Real: real1})
	assert.True(t, found)

	// Key with service1 and real2 should not be found.
	found = state.Lookup(key.Balancer{Service: service1, Real: real2})
	assert.False(t, found)
}

// TestState_Subscribe_Basic tests the basic functionality of the Subscribe
// method. It verifies that a subscription is notified after the state is
// updated.
func TestState_Subscribe_Basic(t *testing.T) {
	state := NewState()

	var wg sync.WaitGroup

	wg.Add(1)
	// Start a goroutine to wait for subscription notification.
	go func() {
		defer wg.Done()
		<-state.Subscribe() // Wait for the notification signal
	}()

	// Allow some time for the subscription to be set up.
	time.Sleep(100 * time.Millisecond)
	// Update the state, which should trigger the subscription notification.
	state.Update(nil)

	// Wait for the goroutine to complete.
	wg.Wait()
}

// TestState_Subscribe_TwoSubscription tests that multiple subscribers to the
// Subscribe method receive notifications when the state is updated.
func TestState_Subscribe_TwoSubscription(t *testing.T) {
	state := NewState()

	var wg sync.WaitGroup

	// Start the first goroutine to wait for the first subscription
	// notification.
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-state.Subscribe() // Wait for the notification signal
	}()

	// Start the second goroutine to wait for the second subscription
	// notification.
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-state.Subscribe() // Wait for the notification signal
	}()

	// Allow some time for the subscriptions to be set up.
	time.Sleep(100 * time.Millisecond)
	// Update the state, which should trigger notifications for all subscribers.
	state.Update(nil)

	// Wait for both goroutines to complete.
	wg.Wait()
}
