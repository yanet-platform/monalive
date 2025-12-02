package real

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yanet-platform/monalive/internal/monitoring/xlog"
	"github.com/yanet-platform/monalive/internal/types/port"
	"github.com/yanet-platform/monalive/internal/types/weight"
	"github.com/yanet-platform/monalive/internal/types/xevent"
)

// testHandler is a test implementation of the xevent.Handler interface. It
// stores the last event passed to the Handle method.
type testHandler struct {
	event *xevent.Event
}

// Handle saves the given event in the structure for later use.
func (m *testHandler) Handle(event *xevent.Event) {
	m.event = event
}

// Event returns the stored event and then resets it to nil.
func (m *testHandler) Event() *xevent.Event {
	defer func() { m.event = nil }()
	return m.event
}

// defaultReal creates a default Real instance with the given weight and event
// handler.
func defaultReal(weight weight.Weight, handler xevent.Handler) *Real {
	config := &Config{
		IP:               netip.MustParseAddr("127.0.0.1"),
		Port:             port.Port(80),
		InhibitOnFailure: false,
		Weight:           weight,
	}

	return New(config, handler, xlog.NewNopLogger())
}

// WithInhibitOnFailure enables the InhibitOnFailure option in the Real's
// configuration.
func (m *Real) WithInhibitOnFailure() *Real {
	m.config.InhibitOnFailure = true
	return m
}

// WithDynWeight enables dynamic weight adjustment for the Real instance.
func (m *Real) WithDynWeight() *Real {
	m.state.DynWeight = true
	return m
}

// WithoutDynWeight disables dynamic weight adjustment for the Real instance.
func (m *Real) WithoutDynWeight() *Real {
	m.state.DynWeight = false
	return m
}

// enableEvent creates an [xevent.Event] to enable a real with the specified
// weight. The event type is set to [xevent.Enable].
func enableEvent(weight weight.Weight) *xevent.Event {
	return &xevent.Event{
		Type: xevent.Enable,
		New: xevent.Status{
			Weight: weight,
		},
	}
}

// disableEvent creates an [xevent.Event] to disable a real.
// The event type is set to [xevent.Disable] and the weight is set to omitted.
func disableEvent() *xevent.Event {
	return &xevent.Event{
		Type: xevent.Disable,
		New: xevent.Status{
			Weight: weight.Omitted,
		},
	}
}

// shutdownEvent creates an [xevent.Event] to shut down a real.
// The event type is set to [xevent.Shutdown] and the weight is set to omitted.
func shutdownEvent() *xevent.Event {
	return &xevent.Event{
		Type: xevent.Shutdown,
		New: xevent.Status{
			Weight: weight.Omitted,
		},
	}
}

// TestHandleEvent_EnableDisabled tests the scenario where a Real instance is
// enabled after being initially disabled. It verifies that the weight and state
// are correctly updated and that an enable event is correctly handled and
// stored.
func TestHandleEvent_EnableDisabled(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	real := defaultReal(initWeight, handler.Handle)

	{
		// Enable the Real instance with omitted weight, and check its state.
		check := enableEvent(weight.Omitted)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, true, state.Alive)

		// Verify that the enable event was handled and that it contains the
		// correct information.
		event := handler.Event()
		require.NotNil(t, event)
		assert.Equal(t, xevent.Enable, event.Type)
		assert.Equal(t, real.Key(), event.Real)
		assert.Equal(t, true, event.New.Enable)
		assert.Equal(t, initWeight, event.New.Weight)
		assert.Equal(t, false, event.Init.Enable)
		assert.Equal(t, weight.Weight(0), event.Init.Weight)
	}
}

// TestHandleEvent_EnableEnabled tests the scenario where a Real instance is
// enabled when it is already enabled. It verifies that no redundant event is
// handled or stored on repeated enable attempts.
func TestHandleEvent_EnableEnabled(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	real := defaultReal(initWeight, handler.Handle)

	{
		check := enableEvent(weight.Omitted)
		real.HandleEvent(check)
		require.NotNil(t, handler.Event())
	}

	{
		// Re-enable the Real instance and verify that no additional event is
		// handled.
		check := enableEvent(weight.Omitted)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, true, state.Alive)

		require.Nil(t, handler.Event())
	}
}

// TestHandleEvent_DisableDisabled tests the scenario where a Real instance is
// disabled when it is already disabled. It verifies that no event is handled or
// stored, and that the state remains consistent.
func TestHandleEvent_DisableDisabled(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	real := defaultReal(initWeight, handler.Handle)

	{
		// Disable the Real instance and check its state.
		check := disableEvent()
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, weight.Weight(0), state.Weight)
		assert.Equal(t, false, state.Alive)

		// Verify that no event was handled.
		require.Nil(t, handler.Event())
	}
}

// TestHandleEvent_DisableEnabled tests the scenario where a Real instance is
// disabled after being enabled. It verifies that the state and weight are
// correctly updated, and that a disable event is handled and stored.
func TestHandleEvent_DisableEnabled(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	real := defaultReal(initWeight, handler.Handle)

	{
		// Enable the Real instance and verify that an event is handled.
		check := enableEvent(weight.Omitted)
		real.HandleEvent(check)
		require.NotNil(t, handler.Event())
	}

	{
		// Disable the Real instance and check its state.
		check := disableEvent()
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, false, state.Alive)

		// Verify that the disable event was handled and contains the correct
		// information.
		event := handler.Event()
		require.NotNil(t, event)
		assert.Equal(t, xevent.Disable, event.Type)
		assert.Equal(t, real.Key(), event.Real)
		assert.Equal(t, false, event.New.Enable)
		assert.Equal(t, weight.Omitted, event.New.Weight)
		assert.Equal(t, true, event.Init.Enable)
		assert.Equal(t, initWeight, event.Init.Weight)
	}
}

// TestHandleEvent_DynWeight_ON tests the scenario where dynamic weight
// adjustment is enabled for a Real instance. It verifies that the weight is
// updated dynamically based on subsequent enable events and that the correct
// event is handled and stored.
func TestHandleEvent_DynWeight_ON(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	real := defaultReal(initWeight, handler.Handle).WithDynWeight()

	{
		// Enable the Real instance with dynamic weight and verify that an event
		// is handled.
		check := enableEvent(initWeight)
		real.HandleEvent(check)
		require.NotNil(t, handler.Event())
	}

	{
		// Change the weight dynamically and verify that the state and event
		// reflect the new weight.
		newWeight := weight.Weight(10)
		check := enableEvent(newWeight)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, newWeight, state.Weight)
		assert.Equal(t, true, state.Alive)

		// Verify that the enable event was handled and contains the correct
		// dynamic weight information.
		event := handler.Event()
		require.NotNil(t, event)
		assert.Equal(t, xevent.Enable, event.Type)
		assert.Equal(t, real.Key(), event.Real)
		assert.Equal(t, true, event.New.Enable)
		assert.Equal(t, newWeight, event.New.Weight)
		assert.Equal(t, true, event.Init.Enable)
		assert.Equal(t, initWeight, event.Init.Weight)
	}
}

// TestHandleEvent_DynWeight_OFF tests the scenario where dynamic weight
// adjustment is disabled for a Real instance. It verifies that the weight does
// not change on subsequent enable events and that no redundant event is handled
// or stored.
func TestHandleEvent_DynWeight_OFF(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	real := defaultReal(initWeight, handler.Handle)

	{
		// Enable the Real instance and verify that an event is handled.
		check := enableEvent(initWeight)
		real.HandleEvent(check)
		require.NotNil(t, handler.Event())
	}

	{
		// Attempt to change the weight, but since dynamic weight is off, the
		// weight remains unchanged.
		newWeight := weight.Weight(10)
		check := enableEvent(newWeight)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, true, state.Alive)

		// Verify that no additional event was handled.
		require.Nil(t, handler.Event())
	}
}

// TestHandleEvent_DynWeight_ONOFF verifies that when dynamic weight is enabled,
// the weight can be updated correctly via events, and when dynamic weight is
// disabled, the initial weight is retained, and the proper event is generated.
func TestHandleEvent_DynWeight_ONOFF(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	real := defaultReal(initWeight, handler.Handle).WithDynWeight()

	{
		// Check that enabling the dynamic weight updates the weight correctly.
		newWeight := weight.Weight(10)
		check := enableEvent(newWeight)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, newWeight, state.Weight)
		assert.Equal(t, true, state.Alive)

		require.NotNil(t, handler.Event())
	}

	real.WithoutDynWeight()

	{
		// Check that disabling dynamic weight keeps the initial weight and
		// generates the appropriate event.
		newWeight := weight.Weight(10)
		check := enableEvent(newWeight)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, true, state.Alive)

		event := handler.Event()
		require.NotNil(t, event)
		assert.Equal(t, xevent.Enable, event.Type)
		assert.Equal(t, real.Key(), event.Real)
		assert.Equal(t, true, event.New.Enable)
		assert.Equal(t, initWeight, event.New.Weight)
		assert.Equal(t, true, event.Init.Enable)
		assert.Equal(t, weight.Weight(10), event.Init.Weight)
	}
}

// TestHandleEvent_DynWeight_EmptyWeight tests the scenario where an empty
// weight is passed to the event after a non-empty weight was set dynamically.
// It ensures that the weight remains unchanged and no event is generated.
func TestHandleEvent_DynWeight_EmptyWeight(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	real := defaultReal(initWeight, handler.Handle).WithDynWeight()

	{
		// Set a new weight and check that it is applied correctly.
		newWeight := weight.Weight(10)
		check := enableEvent(newWeight)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, newWeight, state.Weight)
		assert.Equal(t, true, state.Alive)

		require.NotNil(t, handler.Event())
	}

	{
		// Pass an empty weight and check that the previous weight is retained.
		newWeight := weight.Omitted
		check := enableEvent(newWeight)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, weight.Weight(10), state.Weight)
		assert.Equal(t, true, state.Alive)

		require.Nil(t, handler.Event())
	}
}

// TestHandleEvent_Inhibit verifies that when the inhibit feature is enabled, a
// disable event properly sets the state to inhibited and updates the internal
// state and event correctly.
func TestHandleEvent_Inhibit(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	real := defaultReal(initWeight, handler.Handle).WithInhibitOnFailure()

	{
		// Check that enabling the real sets it to alive with the correct
		// weight.
		check := enableEvent(initWeight)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, true, state.Alive)

		require.NotNil(t, handler.Event())
	}

	{
		// Check that disabling the real sets it to inhibited with the proper
		// event generated.
		check := disableEvent()
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, false, state.Alive)
		assert.Equal(t, true, state.Inhibited)

		event := handler.Event()
		require.NotNil(t, event)
		assert.Equal(t, xevent.Enable, event.Type)
		assert.Equal(t, real.Key(), event.Real)
		assert.Equal(t, true, event.New.Enable)
		assert.Equal(t, weight.Weight(0), event.New.Weight)
		assert.Equal(t, true, event.Init.Enable)
		assert.Equal(t, initWeight, event.Init.Weight)
	}
}

// TestHandleEvent_Inhibit_OFFON verifies that when the inhibit feature is
// turned on after a disable event, the real correctly transitions to the
// inhibited state, and events reflect the change accurately.
func TestHandleEvent_Inhibit_OFFON(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	real := defaultReal(initWeight, handler.Handle)

	{
		// Enable the real and check that it's set to alive with the correct
		// weight.
		check := enableEvent(initWeight)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, true, state.Alive)

		require.NotNil(t, handler.Event())
	}

	{
		// Disable the real and ensure it is properly marked as disabled.
		check := disableEvent()
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, false, state.Alive)

		require.NotNil(t, handler.Event())
	}

	real.WithInhibitOnFailure()

	{
		// Ensure that disabling the real again while inhibited updates the
		// state and events correctly.
		check := disableEvent()
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, false, state.Alive)
		assert.Equal(t, true, state.Inhibited)

		event := handler.Event()
		require.NotNil(t, event)
		assert.Equal(t, xevent.Enable, event.Type)
		assert.Equal(t, real.Key(), event.Real)
		assert.Equal(t, true, event.New.Enable)
		assert.Equal(t, weight.Weight(0), event.New.Weight)
		assert.Equal(t, false, event.Init.Enable)
		assert.Equal(t, initWeight, event.Init.Weight)
	}
}

// TestHandleEvent_Inhibit_EnableInhibited verifies that when an inhibited real
// is re-enabled, it correctly transitions out of the inhibited state and the
// corresponding event is generated.
func TestHandleEvent_Inhibit_EnableInhibited(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	real := defaultReal(initWeight, handler.Handle).WithInhibitOnFailure()

	{
		// Enable the real and check that it is properly set to alive with the
		// initial weight.
		check := enableEvent(initWeight)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, true, state.Alive)

		require.NotNil(t, handler.Event())
	}

	{
		// Disable the real and verify that it transitions to the inhibited
		// state.
		check := disableEvent()
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, false, state.Alive)
		assert.Equal(t, true, state.Inhibited)

		require.NotNil(t, handler.Event())
	}

	{
		// Re-enable the real and verify that it correctly exits the inhibited
		// state.
		check := enableEvent(initWeight)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, true, state.Alive)
		assert.Equal(t, false, state.Inhibited)

		event := handler.Event()
		require.NotNil(t, event)
		assert.Equal(t, xevent.Enable, event.Type)
		assert.Equal(t, real.Key(), event.Real)
		assert.Equal(t, true, event.New.Enable)
		assert.Equal(t, initWeight, event.New.Weight)
		assert.Equal(t, true, event.Init.Enable)
		assert.Equal(t, weight.Weight(0), event.Init.Weight)
	}
}

// TestHandleEvent_Inhibit_EnableInhibitedWithDynWeight checks that a real with
// dynamic weight and inhibit on failure transitions correctly when re-enabled,
// including updating the weight and generating the proper event.
func TestHandleEvent_Inhibit_EnableInhibitedWithDynWeight(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	real := defaultReal(initWeight, handler.Handle).WithInhibitOnFailure().WithDynWeight()

	{
		// Enable the real and verify that it becomes alive with the initial
		// weight.
		check := enableEvent(initWeight)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, true, state.Alive)

		require.NotNil(t, handler.Event())
	}

	{
		// Disable the real and check that it becomes inhibited with the initial
		// weight.
		check := disableEvent()
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, false, state.Alive)
		assert.Equal(t, true, state.Inhibited)

		require.NotNil(t, handler.Event())
	}

	{
		// Re-enable the real with a new weight and verify it exits the
		// inhibited state.
		newWeight := weight.Weight(10)
		check := enableEvent(newWeight)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, newWeight, state.Weight)
		assert.Equal(t, true, state.Alive)
		assert.Equal(t, false, state.Inhibited)

		event := handler.Event()
		require.NotNil(t, event)
		assert.Equal(t, xevent.Enable, event.Type)
		assert.Equal(t, real.Key(), event.Real)
		assert.Equal(t, true, event.New.Enable)
		assert.Equal(t, newWeight, event.New.Weight)
		assert.Equal(t, true, event.Init.Enable)
		assert.Equal(t, weight.Weight(0), event.Init.Weight)
	}
}

// TestHandleEvent_Inhibit_DisableInhibited tests the scenario where a Real
// instance with inhibition enabled is disabled. It verifies that the state
// transitions to inhibited and that subsequent disable events do not trigger
// additional event handling.
func TestHandleEvent_Inhibit_DisableInhibited(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	real := defaultReal(initWeight, handler.Handle).WithInhibitOnFailure()

	{
		// Enable the Real instance and verify its state.
		check := enableEvent(initWeight)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, true, state.Alive)

		// Verify that the enable event was handled and stored.
		require.NotNil(t, handler.Event())
	}

	{
		// Disable the Real instance and verify its state transitions to
		// inhibited.
		check := disableEvent()
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, false, state.Alive)
		assert.Equal(t, true, state.Inhibited)

		// Verify that the disable event was handled and stored.
		require.NotNil(t, handler.Event())
	}

	{
		// Disable the Real instance again and verify that the state remains
		// inhibited.
		check := disableEvent()
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, false, state.Alive)
		assert.Equal(t, true, state.Inhibited)

		// Verify that no additional event was handled.
		require.Nil(t, handler.Event())
	}
}

// TestHandleEvent_Shutdown_Enabled tests the scenario where a Real instance is
// shut down while it is enabled. It verifies that the shutdown event is
// correctly handled and contains the correct initial and new state information.
func TestHandleEvent_Shutdown_Enabled(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	real := defaultReal(initWeight, handler.Handle)

	{
		// Enable the Real instance and verify its state.
		check := enableEvent(initWeight)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, true, state.Alive)

		// Verify that the enable event was handled and stored.
		require.NotNil(t, handler.Event())
	}

	{
		// Shutdown the Real instance and verify that the shutdown event is
		// handled.
		check := shutdownEvent()
		real.HandleEvent(check)

		assert.Equal(t, false, real.State().Alive)
		assert.Equal(t, initWeight, real.State().Weight)

		event := handler.Event()
		require.NotNil(t, event)
		assert.Equal(t, real.Key(), event.Real)
		assert.Equal(t, xevent.Shutdown, event.Type)
		assert.Equal(t, true, event.Init.Enable)
		assert.Equal(t, initWeight, event.Init.Weight)
	}
}

// TestHandleEvent_Shutdown_Disabled tests the scenario where a Real instance is
// shutdown while it is disabled. It verifies that no shutdown event is
// generated when the instance is already disabled.
func TestHandleEvent_Shutdown_Disabled(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	real := defaultReal(initWeight, handler.Handle)

	{
		// Shut down the Real instance while it is disabled.
		check := shutdownEvent()
		real.HandleEvent(check)

		// Verify that no shutdown event was handled.
		event := handler.Event()
		require.Nil(t, event)
	}
}

// TestHandleEvent_Shutdown_Inhibited tests the scenario where a Real instance
// with inhibition enabled is shutdown. It verifies that the shutdown event is
// correctly handled, even when the instance is inhibited.
func TestHandleEvent_Shutdown_Inhibited(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	real := defaultReal(initWeight, handler.Handle).WithInhibitOnFailure()

	{
		// Enable the Real instance and verify its state.
		check := enableEvent(initWeight)
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, true, state.Alive)

		// Verify that the enable event was handled and stored.
		require.NotNil(t, handler.Event())
	}

	{
		// Disable the Real instance and verify its state transitions to
		// inhibited.
		check := disableEvent()
		real.HandleEvent(check)
		state := real.State()
		assert.Equal(t, initWeight, state.Weight)
		assert.Equal(t, false, state.Alive)
		assert.Equal(t, true, state.Inhibited)

		// Verify that the disable event was handled and stored.
		require.NotNil(t, handler.Event())
	}

	{
		// Shutdown the Real instance and verify that the shutdown event is
		// handled.
		check := shutdownEvent()
		real.HandleEvent(check)

		assert.Equal(t, false, real.State().Alive)
		assert.Equal(t, initWeight, real.State().Weight)

		event := handler.Event()
		require.NotNil(t, event)
		assert.Equal(t, real.Key(), event.Real)
		assert.Equal(t, xevent.Shutdown, event.Type)
		assert.Equal(t, true, event.Init.Enable)
		assert.Equal(t, weight.Weight(0), event.Init.Weight)
	}
}
