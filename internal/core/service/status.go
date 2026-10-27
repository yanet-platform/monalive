package service

import (
	"net/netip"

	"github.com/yanet-platform/monalive/internal/core/real"
	"github.com/yanet-platform/monalive/pkg/optport"
	"github.com/yanet-platform/monalive/pkg/weight"
)

// Status is a snapshot of a service and its reals.
type Status struct {
	VIP                    netip.Addr
	Port                   optport.Port
	Protocol               string
	LVSMethod              string
	QuorumState            bool
	AliveWeight            weight.Weight
	AliveCount             int
	Transitions            int
	FWMark                 int
	AnnounceGroup          string
	IPv4OuterSourceNetwork string
	IPv6OuterSourceNetwork string
	Reals                  []real.Status
	QuorumUp               string
}

// Status retrieves the current service state and its real snapshots.
func (m *Service) Status() Status {
	m.realsMu.Lock()
	defer m.realsMu.Unlock()

	state := m.State()

	reals := make([]real.Status, 0, len(m.reals))
	for _, real := range m.reals {
		reals = append(reals, real.Status())
	}

	return Status{
		VIP:                    m.config.VIP,
		Port:                   m.config.VPort,
		Protocol:               m.config.Protocol,
		LVSMethod:              m.config.ForwardingMethod,
		QuorumState:            state.Alive,
		AliveWeight:            state.Weight,
		AliveCount:             state.RealsAlive,
		Transitions:            state.Transitions,
		FWMark:                 m.config.FwMark,
		AnnounceGroup:          m.config.AnnounceGroup,
		IPv4OuterSourceNetwork: m.config.IPv4OuterSourceNetwork,
		IPv6OuterSourceNetwork: m.config.IPv6OuterSourceNetwork,
		Reals:                  reals,
		QuorumUp:               m.config.QuorumUp,
	}
}
