package service

import (
	"log/slog"

	"monalive/internal/types/weight"
	"monalive/internal/types/xevent"
)

// HandleEvent handles an event sent from the real. Depending on the event type,
// it updates the service state and notifies the balancer and announcer if
// necessary.
func (m *Service) HandleEvent(event *xevent.Event) {
	// Increment the wait group counter for event processing.
	// Service won't be stopped until the wait group counter is zero.
	m.eventsWG.Add(1)
	defer m.eventsWG.Done()

	// Process the event.
	announceChanged := m.processEvent(event)

	// Real sends event only when the status changes (enable, disable or weight
	// changes). All these changes must be synced with the load balancer.
	m.balancer.HandleEvent(event)

	if announceChanged && m.config.AnnounceGroup != "" {
		// If the service announce status changed and service's announce group
		// is set (which means that the service affects it's host prefix
		// announce), then pass the update to the announcer.
		err := m.announcer.UpdateService(m.config.AnnounceGroup, m.key, m.state.Alive)
		if err != nil {
			m.log.Error("failed to set up announce", slog.Any("error", err))
		}
	}
}

// processEvent processes an event received by the service.
func (m *Service) processEvent(event *xevent.Event) (announceChanged bool) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	// Assign the service key to the event.
	event.Service = m.key

	// Process the event based on its type.
	switch event.Type {
	case xevent.Enable:
		m.processSucceed(event)
	case xevent.Disable, xevent.Shutdown:
		m.processFailure(event)
	}

	// Update the service announce status.
	changed := m.updateAnnounce()
	return changed
}

// processSucceed updates the service state when a successful event occurs.
// It adjusts the total weight and increments the count of active reals.
func (m *Service) processSucceed(event *xevent.Event) {
	// Calculate the weight change from the event.
	delta := event.New.Weight - event.Init.Weight
	if !event.Init.Enable {
		// If the initial state was disabled, use the new weight directly.
		delta = event.New.Weight
		// Also, if the real was previoulsy disabled increment the count of
		// alive reals.
		m.state.RealsAlive++
	}
	// Update the total weight of the service.
	m.state.Weight += delta
}

// processFailure updates the service state when a failure event occurs. It
// adjusts the total weight and decrements the count of active reals.
func (m *Service) processFailure(event *xevent.Event) {
	// Disable events does not contains new weight. Weight of disabled real is
	// stored in the event.Init status.
	m.state.Weight -= event.Init.Weight
	// Decrement the count of alive reals.
	m.state.RealsAlive--
}

// updateAnnounce determines the new service state based on quorum and updates
// the service announcement accordingly. Returns true if the state changed.
func (m *Service) updateAnnounce() (changed bool) {
	// Determine the new service state based on quorum.
	newServiceState := m.quorumState()

	m.log.Info("service alive update",
		slog.String("service_state", newServiceState.String()),
		slog.Bool("alive", m.state.Alive),
		slog.Int("alive_count", m.state.RealsAlive),
		slog.Int("weight", int(m.state.Weight)),
		slog.String("event_type", "service update"),
	)

	// Enable or disable the service based on the new state.
	switch newServiceState {
	case quorumUp:
		// Enable the service if the quorum is up.
		return m.enableService()
	case quorumDown:
		// Disable the service if the quorum is down.
		return m.disableService()
	default:
		// No change if the quorum holds old status.
		return false
	}
}

// enableService enables the service if its currently disabled.
// Returns true if the service state was changed.
func (m *Service) enableService() (changed bool) {
	if m.state.Alive {
		// No change if the service is already enabled.
		return false
	}

	// Mark the service as enabled.
	m.state.Alive = true
	// Increment the transition counter.
	m.state.Transitions++

	m.log.Info(
		"service enabled",
		slog.Int("quorum", m.config.Quorum),
		slog.Int("hysteresis", m.config.Hysteresis),
		slog.String("event_type", "service update"),
	)
	// Indicate that the state was changed.
	return true
}

// disableService disables the service if its currently enabled.
// Returns true if the service state was changed.
func (m *Service) disableService() (changed bool) {
	if !m.state.Alive {
		// No change if the service is already disabled.
		return false
	}

	// Mark the service disabled.
	m.state.Alive = false
	// Increment the transition counter.
	m.state.Transitions++

	m.log.Info(
		"service disabled",
		slog.Int("quorum", m.config.Quorum),
		slog.Int("hysteresis", m.config.Hysteresis),
		slog.String("event_type", "service update"),
	)
	// Indicate that the state was changed.
	return true
}

// quorumState calculates the current quorum state based on the service weight
// and configured quorum thresholds. Returns the appropriate quorum state.
func (m *Service) quorumState() quorum {
	// Get the current weight of the service.
	w := m.state.Weight
	// Retrieve quorum and hysteresis values from the config.
	quorum, hysteresis := m.config.Quorum, m.config.Hysteresis

	// Determine the quorum state based on the weight.
	switch {
	case w >= weight.Weight(quorum+hysteresis):
		// If the weight exceeds the quorum + hysteresis, the quorum is up.
		return quorumUp
	case w < weight.Weight(quorum-hysteresis) || w == 0:
		// If the weight is below the quorum - hysteresis or zero, the quorum is
		// down.
		return quorumDown
	default:
		// Otherwise, hold the current quorum state.
		return quorumHold
	}
}

// quorum determines whether raise or remove the announce.
type quorum uint8

const (
	// quorumHold indicates holding the current announce state.
	quorumHold quorum = iota
	// quorumUp indicates raising the announce (enabling the service).
	quorumUp
	// quorumDown indicates removing the announce (disabling the service).
	quorumDown
)

func (m quorum) String() string {
	switch m {
	case quorumUp:
		return "up"
	case quorumDown:
		return "down"
	default:
		return "hold"
	}
}
