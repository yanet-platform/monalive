package balancer

import (
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	log "go.uber.org/zap"

	"github.com/yanet-platform/monalive/internal/types/key"
	"github.com/yanet-platform/monalive/internal/types/xevent"
)

// mockBalancerClient is a mock implementation of the LoadBalancerClient
// interface used in testing. It does not add any functionality but allows for
// easier testing of the Balancer struct.
type mockBalancerClient struct {
	LoadBalancerClient
}

// mockBalancerClientWithStates is a mock implementation of the
// LoadBalancerClient and Stater interfaces used in testing. It allows for
// testing with a state-aware balancer client.
type mockBalancerClientWithStates struct {
	LoadBalancerClient
	Stater
}

// defaultKey returns a default key for testing purposes, with preset values for
// service and real addresses.
func defaultKey() key.Balancer {
	return key.Balancer{
		Service: key.Service{
			Addr:  netip.MustParseAddr("127.0.0.1"),
			Port:  80,
			Proto: "TCP",
		},
		Real: key.Real{
			Addr: netip.MustParseAddr("127.0.0.2"),
			Port: 80,
		},
	}
}

// TestBalancer_HandleEvent_Basic tests the basic functionality of the
// HandleEvent method of the Balancer struct. It verifies that an event is
// correctly handled and stored in the balancer's events map with the correct
// new and initial status.
func TestBalancer_HandleEvent_Basic(t *testing.T) {
	balancer := New(&Config{}, &mockBalancerClient{}, nil, log.NewNop())

	// Define a default key and initial status.
	key := defaultKey()
	initStatus := xevent.Status{Enable: true, Weight: 90}

	// Define new status for the key.
	newStatus := xevent.Status{Enable: true, Weight: 100}
	{
		// Create an event with the new and initial status.
		event := &xevent.Event{Balancer: key, New: newStatus, Init: initStatus}
		balancer.HandleEvent(event)

		// Verify the event has been recorded correctly.
		events := balancer.events.Events()
		require.Contains(t, events, key)
		assert.Equal(t, events[key].New, newStatus)
		assert.Equal(t, events[key].Init, initStatus)
	}
}

// TestBalancer_HandleEvent_Update tests the HandleEvent method of the Balancer
// struct to ensure it updates the event status correctly. It verifies that the
// new status is recorded after multiple updates.
func TestBalancer_HandleEvent_Update(t *testing.T) {
	balancer := New(&Config{}, &mockBalancerClient{}, nil, log.NewNop())

	// Define a default key and initial status.
	key := defaultKey()
	initStatus := xevent.Status{Enable: true, Weight: 90}

	// Define new status for the key and update it.
	oldStatus := initStatus
	newStatus := xevent.Status{Enable: true, Weight: 100}
	{
		// Create an event with the new and initial status.
		event := &xevent.Event{Balancer: key, New: newStatus, Init: oldStatus}
		balancer.HandleEvent(event)

		// Verify the event has been recorded correctly.
		events := balancer.events.Events()
		require.Contains(t, events, key)
		assert.Equal(t, events[key].New, newStatus)
		assert.Equal(t, events[key].Init, initStatus)
	}

	// Update the status again.
	oldStatus = newStatus
	newStatus = xevent.Status{Enable: true, Weight: 110}
	{
		// Create another event with the updated status.
		event := &xevent.Event{Balancer: key, New: newStatus, Init: oldStatus}
		balancer.HandleEvent(event)

		// Verify the updated event has been recorded correctly.
		events := balancer.events.Events()
		require.Contains(t, events, key)
		assert.Equal(t, events[key].New, newStatus)
		assert.Equal(t, events[key].Init, initStatus)
	}
}

// TestBalancer_HandleEvent_RemoveEnable tests the HandleEvent method for
// scenarios where the event status is updated to remove enablement. It verifies
// that the event is correctly removed from the balancer's events map when the
// status is changed to disablement.
func TestBalancer_HandleEvent_RemoveEnable(t *testing.T) {
	balancer := New(&Config{}, &mockBalancerClient{}, nil, log.NewNop())

	// Define a default key and initial status.
	key := defaultKey()
	initStatus := xevent.Status{Enable: true, Weight: 90}

	// Define new status for the key.
	oldStatus := initStatus
	newStatus := xevent.Status{Enable: true, Weight: 100}
	{
		// Create an event with the new and initial status.
		event := &xevent.Event{Balancer: key, New: newStatus, Init: oldStatus}
		balancer.HandleEvent(event)

		// Verify the event has been recorded correctly.
		events := balancer.events.Events()
		require.Contains(t, events, key)
		assert.Equal(t, events[key].New, newStatus)
		assert.Equal(t, events[key].Init, initStatus)
	}

	// Update the status to remove enablement.
	oldStatus = newStatus
	newStatus = xevent.Status{Enable: true, Weight: 90}
	{
		// Create an event with the updated status.
		event := &xevent.Event{Balancer: key, New: newStatus, Init: oldStatus}
		balancer.HandleEvent(event)

		// Verify the event is removed from the balancer's events map.
		events := balancer.events.Events()
		assert.NotContains(t, events, key)
	}
}

// TestBalancer_HandleEvent_RemoveDisable tests the HandleEvent method for
// scenarios where the event status is updated to disable the key. It verifies
// that the event is correctly removed from the balancer's events map when the
// status is updated to disablement.
func TestBalancer_HandleEvent_RemoveDisable(t *testing.T) {
	balancer := New(&Config{}, &mockBalancerClient{}, nil, log.NewNop())

	// Define a default key and initial status.
	key := defaultKey()
	initStatus := xevent.Status{Enable: true, Weight: 90}

	// Define new status for the key.
	oldStatus := initStatus
	newStatus := xevent.Status{Enable: false, Weight: 0}
	{
		// Create an event with the new and initial status.
		event := &xevent.Event{Balancer: key, New: newStatus, Init: oldStatus}
		balancer.HandleEvent(event)

		// Verify the event has been recorded correctly.
		events := balancer.events.Events()
		require.Contains(t, events, key)
		assert.Equal(t, events[key].New, newStatus)
		assert.Equal(t, events[key].Init, initStatus)
	}

	// Update the status to re-enable the key.
	oldStatus = newStatus
	newStatus = xevent.Status{Enable: true, Weight: 90}
	{
		// Create another event with the updated status.
		event := &xevent.Event{Balancer: key, New: newStatus, Init: oldStatus}
		balancer.HandleEvent(event)

		// Verify the event is removed from the balancer's events map.
		events := balancer.events.Events()
		assert.NotContains(t, events, key)
	}
}

// TestBalancer_LookupSubscription_KeyExists tests the LookupSubscription method
// when the key already exists in the balancer state. It verifies that the
// subscription returns nil, indicating no notification is needed.
func TestBalancer_LookupSubscription_KeyExists(t *testing.T) {
	balancer := New(&Config{}, &mockBalancerClientWithStates{}, nil, log.NewNop())

	// [BEGIN] Fill up balancer state.
	balancerKey := defaultKey()

	service1 := balancerKey.Service
	service1.Addr = netip.MustParseAddr("127.0.1.1")
	real1 := balancerKey.Real
	real1.Addr = netip.MustParseAddr("127.0.1.2")

	service2 := balancerKey.Service
	service2.Addr = netip.MustParseAddr("127.0.2.1")
	real2 := balancerKey.Real
	real2.Addr = netip.MustParseAddr("127.0.2.2")

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
	balancer.state.Update(update)
	// [END] Fill up balancer state.

	ctx := context.Background()

	// Test subscription lookup for an existing key.
	notify := balancer.LookupSubscription(ctx, key.Balancer{Service: service1, Real: real1})
	assert.Nil(t, notify)
}

// TestBalancer_LookupSubscription_KeyAdded tests the LookupSubscription method
// when a key is added to the balancer state. It verifies that the subscription
// receives a notification when the state is updated.
func TestBalancer_LookupSubscription_KeyAdded(t *testing.T) {
	balancer := New(&Config{}, &mockBalancerClientWithStates{}, nil, log.NewNop())

	// [BEGIN] Construct balancer state update.
	balancerKey := defaultKey()

	service1 := balancerKey.Service
	service1.Addr = netip.MustParseAddr("127.0.1.1")
	real1 := balancerKey.Real
	real1.Addr = netip.MustParseAddr("127.0.1.2")

	service2 := balancerKey.Service
	service2.Addr = netip.MustParseAddr("127.0.2.1")
	real2 := balancerKey.Real
	real2.Addr = netip.MustParseAddr("127.0.2.2")

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
	// [END] Construct balancer state update.

	var wg sync.WaitGroup

	// In this test it is expected that the context won't be canceled.
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		// Test subscription lookup for a key that will be added.
		notify := balancer.LookupSubscription(ctx, key.Balancer{Service: service1, Real: real1})
		require.NotNil(t, notify)
		<-notify
	}()

	// Simulate some delay before updating the state.
	time.Sleep(time.Second)

	// Update the balancer state.
	balancer.state.Update(update)

	wg.Wait()
	assert.NoError(t, ctx.Err())
}

// TestBalancer_LookupSubscription_ContextCancel tests the LookupSubscription
// method when the context is canceled before the key is added to the balancer
// state. It verifies that the context cancellation is handled properly.
func TestBalancer_LookupSubscription_ContextCancel(t *testing.T) {
	balancer := New(&Config{}, &mockBalancerClientWithStates{}, nil, log.NewNop())

	notExistingKey := defaultKey()
	notExistingKey.Service.Addr = netip.MustParseAddr("127.0.1.1")

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		// Test subscription lookup for a non-existing key with a canceled
		// context.
		notify := balancer.LookupSubscription(ctx, notExistingKey)
		require.NotNil(t, notify)
		<-notify
	}()

	wg.Wait()
	assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
}

// TestBalancer_LookupSubscription_BalancerStopped tests the LookupSubscription
// method when the balancer is stopped before the key is added to the state. It
// verifies that the context remains valid and no errors are returned after the
// balancer stops.
func TestBalancer_LookupSubscription_BalancerStopped(t *testing.T) {
	balancer := New(&Config{}, &mockBalancerClientWithStates{}, nil, log.NewNop())

	notExistingKey := defaultKey()
	notExistingKey.Service.Addr = netip.MustParseAddr("127.0.1.1")

	var wg sync.WaitGroup
	// In this test it is expected that the context won't be canceled.
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		// Test subscription lookup for a non-existing key with the balancer
		// stopped.
		notify := balancer.LookupSubscription(ctx, notExistingKey)
		require.NotNil(t, notify)
		<-notify
	}()

	// Simulate some delay before stopping the balancer.
	time.Sleep(time.Second)
	balancer.Stop()

	wg.Wait()
	assert.NoError(t, ctx.Err())
}

// TestBalancer_LookupSubscription_WithoutState tests the LookupSubscription
// method when no state is present in the balancer. It verifies that the
// subscription returns nil, indicating no notifications are available for
// non-existing keys.
func TestBalancer_LookupSubscription_WithoutState(t *testing.T) {
	balancer := New(&Config{}, &mockBalancerClient{}, nil, log.NewNop())

	ctx := context.Background()
	// Test subscription lookup for a key when no state is present.
	notify := balancer.LookupSubscription(ctx, defaultKey())
	require.Nil(t, notify)
}
