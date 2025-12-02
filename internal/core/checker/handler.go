package checker

import (
	"errors"
	"log/slog"
	"time"

	"github.com/yanet-platform/monalive/internal/core/checker/check"
	"github.com/yanet-platform/monalive/internal/types/weight"
	"github.com/yanet-platform/monalive/internal/types/xevent"
)

// ProcessCheck processes the results of a health check, updating the internal
// state and generating events based on whether the check succeeded or failed.
func (m *Checker) ProcessCheck(md check.Metadata, opErr error) {
	// Increment the wait group counter for event processing.
	// Checker won't be stopped until the wait group counter is zero.
	m.eventsWG.Add(1)
	defer m.eventsWG.Done()

	switch md.Alive {
	case true:
		// If the check succeeds, opErr will be nil, so no need to pass it to
		// the handler.
		m.processSucceed(md)
	case false:
		// If the check fails, md will contain {false, EmptyWeight}, so no need
		// to pass it to the handler.
		m.processFail(opErr)
	}
}

// processSucceed handles the successful result of a health check.
//
// It enables the checker if it was previously disabled, recalculates the
// weight, and triggers an event if there were any changes in the status or
// weight.
func (m *Checker) processSucceed(md check.Metadata) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	// Update last check timestamp.
	m.state.Timestamp = time.Now()

	// Attempt to enable the checker if it was previously disabled.
	statusChanged := m.enableChecker()

	// Recalculate the weight based on the current and new weight values.
	newWeight := md.Weight.Recalculate(m.state.Weight, m.config.DynamicWeightCoeff)
	weightChanged := m.updateWeight(newWeight)

	if !statusChanged && !weightChanged && !md.Force {
		// If neither the status nor the weight changed, no event is triggered.
		return
	}

	// Reset the ManualChanged flag.
	m.state.ManualChanged = false

	// Trigger an event to notify that the checker has been enabled with the new
	// weight. Other event fields will be filled in when passing the real and
	// service handlers.
	event := &xevent.Event{
		Type: xevent.Enable,
		New: xevent.Status{
			Weight: m.state.Weight,
		},
	}
	m.handler(event)
}

// processFail handles the failed result of a health check.
//
// It disables the checker if the failure threshold has been exceeded, and
// triggers a failure or shutdown event depending on the nature of the error.
func (m *Checker) processFail(opErr error) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	// Update last check timestamp.
	m.state.Timestamp = time.Now()

	// Disable the checker if the failure threshold has been exceeded.
	if statusChanged := m.disableChecker(opErr); !statusChanged {
		// If the checker was already disabled, no event is triggered.
		return
	}

	// Determine the event type based on the error.
	eventType := xevent.Disable
	if errors.Is(opErr, ErrShutdown) {
		eventType = xevent.Shutdown
	}

	// Trigger an event to notify that the checker has been disabled or shutted
	// down.
	event := &xevent.Event{
		Type: eventType,
		New: xevent.Status{
			Weight: weight.Omitted,
		},
	}
	m.handler(event)
}

// enableChecker enables the checker if it was previously disabled. Returns true
// if the checker was successfully enabled, or false if it was already enabled.
func (m *Checker) enableChecker() (changed bool) {
	if m.state.Alive {
		return false
	}

	m.state.Alive = true
	m.state.FailedAttempts = 0
	return true
}

// disableChecker disables the checker if the failure threshold has been
// exceeded.
//
// If the error is a shutdown error, the checker is always disabled. Otherwise,
// it increments the failed attempt counter and only disables the checker if the
// failure threshold is reached.
func (m *Checker) disableChecker(opErr error) (changed bool) {
	if errors.Is(opErr, ErrShutdown) {
		return true
	}

	if exceeded := m.failedAttempt(opErr); !exceeded {
		// If the failure threshold has not been exceeded, do not disable the
		// checker.
		return false
	}

	m.state.Alive = false

	// Each failed check with exceeded failed attempts must be processed by the
	// real, due to possible changes in the InhibitOnFailure parameter during
	// reload, so return changed = true even if the checker was previously
	// disabled.
	return true
}

// failedAttempt increments the failed attempt counter and logs the error.
//
// If the number of failed attempts exceeds the configured threshold, it returns
// true, indicating that the checker should be disabled. Otherwise, it returns
// false.
func (m *Checker) failedAttempt(opErr error) (exceeded bool) {
	m.state.FailedAttempts++
	// TODO: Consider implementing an exponential counter for error logs to
	// prevent verbose logging.
	m.log.Error(
		"check failed",
		slog.Any("error", opErr),
		slog.Int("attempt", m.state.FailedAttempts),
		slog.String("event_type", "checker update"),
	)
	return m.state.FailedAttempts > m.config.GetRetries()
}

// updateWeight updates the checker's weight if dynamic weight adjustment is
// enabled.
//
// If the dynamic weight option is disabled or the weight hasn't changed, it
// returns false. Otherwise, it updates the checker's weight and returns true.
func (m *Checker) updateWeight(weight weight.Weight) (changed bool) {
	if !m.config.DynamicWeight {
		return false
	}

	if m.state.Weight == weight {
		return false
	}
	m.state.Weight = weight
	return true
}
