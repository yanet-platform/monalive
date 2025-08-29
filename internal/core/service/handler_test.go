package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yanet-platform/monalive/internal/announcer"
	"github.com/yanet-platform/monalive/internal/balancer"
	"github.com/yanet-platform/monalive/internal/monitoring/xlog"
	"github.com/yanet-platform/monalive/internal/types/weight"
	"github.com/yanet-platform/monalive/internal/types/xevent"
)

// defaultService initializes and returns a new Service instance
// with default configuration settings.
func defaultService() *Service {
	serviceConfig := &Config{
		Quorum:     1,
		Hysteresis: 0,
	}
	announcer := announcer.New(&announcer.Config{}, nil, xlog.NewNopLogger())
	balancer := balancer.New(&balancer.Config{}, nil, xlog.NewNopLogger())
	return New(serviceConfig, announcer, balancer, xlog.NewNopLogger())
}

// enableRealEvent creates and returns an Event that simulates enabling a real
// server with the specified weight. The event sets the New status as enabled
// with the provided weight and the Init status as disabled with the same
// weight.
func enableRealEvent(weight weight.Weight) *xevent.Event {
	return &xevent.Event{
		Type: xevent.Enable,
		New: xevent.Status{
			Enable: true,
			Weight: weight,
		},
		Init: xevent.Status{
			Enable: false,
			Weight: weight,
		},
	}
}

// enableRealEventWithDynWeight creates and returns an Event that simulates
// enabling a real server with a dynamic weight change. The event changes the
// server's weight from an old value to a new one, while marking the server as
// enabled.
func enableRealEventWithDynWeight(newWeight, oldWeight weight.Weight) *xevent.Event {
	event := enableRealEvent(oldWeight)
	event.New.Weight = newWeight
	return event
}

// updateRealEventWithDynWeight creates and returns an Event that simulates
// updating an already enabled real server's weight from an old value to a new
// one. The event assumes the server was previously enabled and updates its
// weight while keeping it enabled.
func updateRealEventWithDynWeight(newWeight, oldWeight weight.Weight) *xevent.Event {
	event := enableRealEvent(oldWeight)
	event.New.Weight = newWeight
	event.Init.Enable = true // mark the initial status as enabled to simulate an update
	return event
}

// disableRealEvent creates and returns an Event that simulates disabling a real
// server, setting its weight to zero.
func disableRealEvent(oldWeight weight.Weight) *xevent.Event {
	event := enableRealEvent(oldWeight)
	event.Type = xevent.Disable
	event.New.Enable = false
	event.New.Weight = weight.Omitted
	return event
}

// TestHandleEvent_EnableDisabledReal verifies that enabling a previously
// disabled real server instance correctly updates the service state. It ensures
// that the service state reflects the instance being alive, with the correct
// weight and alive count.
func TestHandleEvent_EnableDisabledReal(t *testing.T) {
	service := defaultService()

	{
		weight := weight.Weight(1)
		event := enableRealEvent(weight)
		service.HandleEvent(event)

		// Check that the service is marked as alive, with the expected weight
		// and reals alive count.
		state := service.State()
		assert.Equal(t, true, state.Alive)
		assert.Equal(t, weight, state.Weight)
		assert.Equal(t, 1, state.RealsAlive)
	}
}

// TestHandleEvent_EnableDisabledReal_DynWeight verifies that enabling a
// previously disabled real server instance with a dynamic weight change
// correctly updates the service state. It ensures that the service state
// reflects the new weight and keeps the instance marked as alive.
func TestHandleEvent_EnableDisabledReal_DynWeight(t *testing.T) {
	service := defaultService()

	{
		newWeight := weight.Weight(2)
		oldWeight := weight.Weight(1)
		event := enableRealEventWithDynWeight(newWeight, oldWeight)
		service.HandleEvent(event)

		// Check that the service is marked as alive, with the updated weight.
		state := service.State()
		assert.Equal(t, true, state.Alive)
		assert.Equal(t, newWeight, state.Weight)
		assert.Equal(t, 1, state.RealsAlive)
	}
}

// TestHandleEvent_EnableDisabledReal_DynWeight_ZeroWeight verifies that
// enabling a previously disabled real server instance with a zero weight
// correctly updates the service state. It ensures that the service state marks
// the instance as not alive, with the weight set to zero.
func TestHandleEvent_EnableDisabledReal_DynWeight_ZeroWeight(t *testing.T) {
	service := defaultService()

	{
		newWeight := weight.Weight(0)
		oldWeight := weight.Weight(1)
		event := enableRealEventWithDynWeight(newWeight, oldWeight)
		service.HandleEvent(event)

		// Check that the service is marked as not alive due to zero weight,
		// with the weight updated.
		state := service.State()
		assert.Equal(t, false, state.Alive)
		assert.Equal(t, newWeight, state.Weight)
		assert.Equal(t, 1, state.RealsAlive)
	}
}

// TestHandleEvent_UpdateEnabledReal verifies that updating the weight of an
// already enabled real server instance correctly reflects the change in the
// service state. It ensures that the service state keeps the instance alive and
// updates its weight.
func TestHandleEvent_UpdateEnabledReal(t *testing.T) {
	service := defaultService()

	{
		weight := weight.Weight(1)
		event := enableRealEvent(weight)
		service.HandleEvent(event)
	}

	{
		newWeight := weight.Weight(2)
		oldWeight := weight.Weight(1)
		event := updateRealEventWithDynWeight(newWeight, oldWeight)
		service.HandleEvent(event)

		// Check that the service is marked as alive, with the updated weight.
		state := service.State()
		assert.Equal(t, true, state.Alive)
		assert.Equal(t, newWeight, state.Weight)
		assert.Equal(t, 1, state.RealsAlive)
	}
}

// TestHandleEvent_DisableEnabledReal verifies that disabling a previously
// enabled real server instance correctly updates the service state. It ensures
// that the service state marks the instance as not alive, with the weight set
// to zero.
func TestHandleEvent_DisableEnabledReal(t *testing.T) {
	service := defaultService()

	{
		weight := weight.Weight(1)
		event := enableRealEvent(weight)
		service.HandleEvent(event)
	}

	{
		oldWeight := weight.Weight(1)
		event := disableRealEvent(oldWeight)
		service.HandleEvent(event)

		// Check that the service is marked as not alive, with the weight set to
		// zero.
		state := service.State()
		assert.Equal(t, false, state.Alive)
		assert.Equal(t, weight.Weight(0), state.Weight)
		assert.Equal(t, 0, state.RealsAlive)
	}
}

// TestHandleEvent_QuorumUp verifies that the service remains not alive if the
// quorum is not met, even when a real server instance is enabled with a weight
// slightly below the quorum. It also verifies that the service becomes alive
// once the weight exceeds the quorum.
func TestHandleEvent_QuorumUp(t *testing.T) {
	service := defaultService()
	service.config.Quorum = 5
	service.config.Hysteresis = 1

	{
		weight := weight.Weight(5)
		event := enableRealEvent(weight)
		service.HandleEvent(event)

		// Check that the service is not alive since the weight is equal to the
		// quorum but not above it.
		state := service.State()
		assert.Equal(t, false, state.Alive)
	}

	{
		newWeight := weight.Weight(6)
		oldWeight := weight.Weight(5)
		event := updateRealEventWithDynWeight(newWeight, oldWeight)
		service.HandleEvent(event)

		// Check that the service becomes alive once the weight exceeds the
		// quorum.
		state := service.State()
		assert.Equal(t, true, state.Alive)
		assert.Equal(t, newWeight, state.Weight)
		assert.Equal(t, 1, state.RealsAlive)
	}
}

// TestHandleEvent_QuorumHold verifies that the service remains alive as long as
// the weight is above the hysteresis value, even when the weight decreases. It
// also verifies that the service becomes not alive once the weight falls below
// the quorum minus hysteresis.
func TestHandleEvent_QuorumHold(t *testing.T) {
	service := defaultService()
	service.config.Quorum = 5
	service.config.Hysteresis = 1

	{
		weight := weight.Weight(6)
		event := enableRealEvent(weight)
		service.HandleEvent(event)

		// Check that the service is alive since the weight is met the quorum
		// plus hysteresis.
		state := service.State()
		assert.Equal(t, true, state.Alive)
	}

	{
		newWeight := weight.Weight(5)
		oldWeight := weight.Weight(6)
		event := updateRealEventWithDynWeight(newWeight, oldWeight)
		service.HandleEvent(event)

		// Check that the service remains alive since the weight is still above
		// the quorum minus hysteresis.
		state := service.State()
		assert.Equal(t, true, state.Alive)
		assert.Equal(t, newWeight, state.Weight)
		assert.Equal(t, 1, state.RealsAlive)
	}

	{
		newWeight := weight.Weight(4)
		oldWeight := weight.Weight(5)
		event := updateRealEventWithDynWeight(newWeight, oldWeight)
		service.HandleEvent(event)

		state := service.State()
		assert.Equal(t, true, state.Alive)
		assert.Equal(t, newWeight, state.Weight)
		assert.Equal(t, 1, state.RealsAlive)
	}

	{
		newWeight := weight.Weight(3)
		oldWeight := weight.Weight(4)
		event := updateRealEventWithDynWeight(newWeight, oldWeight)
		service.HandleEvent(event)

		// Check that the service is not alive since the weight is now below the
		// quorum minus hysteresis.
		state := service.State()
		assert.Equal(t, false, state.Alive)
		assert.Equal(t, newWeight, state.Weight)
		assert.Equal(t, 1, state.RealsAlive)
	}
}
