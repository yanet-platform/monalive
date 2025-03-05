package real

import (
	"log/slog"

	"monalive/internal/types/weight"
	"monalive/internal/types/xevent"
)

// HandleEvent handles an event sent from the checker.
func (m *Real) HandleEvent(event *xevent.Event) {
	// Increment the wait group counter for event processing.
	// Real won't be stopped until the wait group counter is zero.
	m.eventsWG.Add(1)
	defer m.eventsWG.Done()

	// Assign the current real's key to the event for tracking.
	event.Real = m.key

	dropEvent := false
	// Handle the event based on its type.
	switch event.Type {
	case xevent.Enable:
		dropEvent = m.processSucceed(event)
	case xevent.Disable:
		dropEvent = m.processFail(event)
	case xevent.Shutdown:
		dropEvent = m.processShutdown(event)
	}

	if dropEvent {
		return
	}

	// Pass the updated event to the service event handler for further
	// processing.
	m.handler(event)
}

// processSucceed handles the enable event, updating the real's status and
// weight.
func (m *Real) processSucceed(event *xevent.Event) (drop bool) {
	// Lock the state mutex to ensure thread-safe updates to the real's state.
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	// Store the initial status of the real for comparison later.
	initStatus := m.state.Status()

	// If the real is inhibited, ensure it is marked as enabled but with a
	// weight of 0.
	if m.state.Inhibited {
		initStatus.Enable = true
		initStatus.Weight = 0
	}

	// Determine the new weight of the real, considering dynamic weight if
	// applicable.
	newWeight := m.handleDynamicWeight(event.New.Weight)
	// Update the weight and check if it has changed.
	weightChanged := m.updateWeight(newWeight)
	// Enable the real and check if its status has changed.
	statusChanged := m.enableReal()

	// If neither the status nor the weight has changed, no further action is
	// needed.
	if !statusChanged && !weightChanged {
		return true
	}

	// Update the event with the new status and initial status.
	event.New = m.state.Status()
	event.Init = initStatus

	return false
}

// processFail handles the disable event, updating the real's status and weight.
func (m *Real) processFail(event *xevent.Event) (drop bool) {
	// Lock the state mutex to ensure thread-safe updates to the real's state.
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	// Store the initial status of the real for comparison later.
	initStatus := m.state.Status()

	// Disable the real and check if its status has changed.
	if statusChanged := m.disableReal(); !statusChanged {
		// If the status hasn't changed, no further action is needed.
		return true
	}

	// If the real is inhibited, change the event type to Enable and set the
	// weight to 0.
	if m.state.Inhibited {
		event.Type = xevent.Enable
		event.New.Enable = true
		event.New.Weight = 0
	}

	// Update the event with the initial status for comparison later.
	event.Init = initStatus

	return false
}

// processShutdown handles the shutdown event, updating the real's status.
func (m *Real) processShutdown(event *xevent.Event) (drop bool) {
	// Lock the state mutex to ensure thread-safe updates to the real's state.
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	// If the real is already disabled and not inhibited, shutdown does nothing.
	if !m.state.Alive && !m.state.Inhibited {
		return true
	}

	// Update the event with the initial status before shutdown.
	event.Init = m.state.Status()

	return false
}

// handleDynamicWeight determines the weight of the real service, considering
// dynamic weight if applicable.
func (m *Real) handleDynamicWeight(weightFromChecker weight.Weight) weight.Weight {
	switch {
	// If dynamic weight is not enabled, return the static config weight.
	case !m.state.DynWeight:
		return m.config.Weight

	// If the weight from the checker is not omitted, use it as the new weight.
	case weightFromChecker != weight.Omitted:
		return weightFromChecker

	// Otherwise, return the current weight from the real's state.
	default:
		return m.state.Weight
	}
}

// enableReal enables the real, marking it as enabled and updating its status.
func (m *Real) enableReal() (changed bool) {
	// If the real is already enabled, no changes are needed.
	if m.state.Alive {
		return false
	}

	// Mark the real as enabled and clear any inhibition.
	m.state.Alive = true
	m.state.Inhibited = false
	// Increment the transition counter to track status changes.
	m.state.Transitions++
	m.log.Info("real enabled", slog.Int("weight", int(m.state.Weight)), slog.String("event_type", "real update"))
	return true
}

// disableReal disables the real, updating its status and inhibition state if
// applicable.
func (m *Real) disableReal() (changed bool) {
	// If the real is already disabled and can't be inhibited, do nothing.
	//
	// NOTE: this might be a bit confusing why checking the InhibitOnFailure
	// configuration parameter is necessary here. This is done to prevent the
	// following situation:
	//  1. The real config loaded with InhibitOnFailure = false.
	//  2. Reload performed with the same real config, but with
	//  the InhibitOnFailure parameter set to true.
	//  3. If this real was disabled, after the reload it is expected that
	//  the real will be enabled in balancer with weight equals 0. So if don't
	//  check this parameter, the described transition will be lost.
	if !m.state.Alive && !m.config.InhibitOnFailure {
		return false
	}

	// If the real is already disabled and inhibited, do nothing.
	if !m.state.Alive && m.state.Inhibited {
		return false
	}

	// If inhibition on failure is configured, mark the real as inhibited.
	if m.config.InhibitOnFailure {
		m.state.Inhibited = true
	}

	// Mark the real as disabled.
	m.state.Alive = false
	// Increment the transition counter to track status changes.
	m.state.Transitions++
	m.log.Info("real disabled", slog.String("event_type", "real update"))
	return true
}

// updateWeight update the weight value of real. Returns true if the new value
// differs from the previous one, otherwise false.
func (m *Real) updateWeight(weight weight.Weight) (changed bool) {
	// If the new weight is the same as the current weight, no changes are
	// needed.
	if m.state.Weight == weight {
		return false
	}

	// Update the weight in the real's state.
	m.state.Weight = weight
	m.log.Info(
		"real weight changed",
		slog.Int("weight", int(m.state.Weight)),
		slog.String("event_type", "real update"),
	)
	return true
}
