package real

import (
	"net/netip"

	"github.com/yanet-platform/monalive/internal/core/checker"
	"github.com/yanet-platform/monalive/pkg/optport"
	"github.com/yanet-platform/monalive/pkg/weight"
)

// Status is a snapshot of a real and its checkers.
type Status struct {
	IP          netip.Addr
	Port        optport.Port
	Alive       bool
	Weight      weight.Weight
	Transitions int
	Checkers    []checker.Status
}

// Status retrieves the current real state and its checker snapshots.
func (m *Real) Status() Status {
	m.checkersMu.Lock()
	defer m.checkersMu.Unlock()

	state := m.State()

	checkers := make([]checker.Status, 0, len(m.checkers))
	for _, checker := range m.checkers {
		checkers = append(checkers, checker.Status())
	}

	return Status{
		IP:          m.key.Addr,
		Port:        m.key.Port,
		Alive:       state.Alive,
		Weight:      state.Weight,
		Transitions: state.Transitions,
		Checkers:    checkers,
	}
}
