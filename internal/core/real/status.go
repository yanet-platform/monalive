package real

import (
	monalivepb "monalive/gen/manager"
)

// Status retrieves the current status of all checkers managed by this real. It
// returns [monalivepb.RealStatus] messages representing the status of the real
// and its checkers.
func (m *Real) Status() *monalivepb.RealStatus {
	// Lock the reals mutex to ensure thread-safe access.
	m.checkersMu.Lock()
	defer m.checkersMu.Unlock()

	state := m.State()

	// Create a slice to hold the statuses of real's checkers.
	checkerStatus := make([]*monalivepb.CheckerStatus, 0, len(m.checkers))
	// Iterate over each checker and append it's status to the slice.
	for _, checker := range m.checkers {
		checkerStatus = append(checkerStatus, checker.Status())
	}

	// Construct real status based on it's state and checkers status slice.
	return &monalivepb.RealStatus{
		Ip:          m.config.IP.String(),
		Port:        m.config.Port.ProtoMarshaller(),
		Alive:       state.Alive,
		Weight:      state.Weight.Uint32(),
		Transitions: uint32(state.Transitions),
		Checkers:    checkerStatus,
	}
}
