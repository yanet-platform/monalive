package service

import (
	monalivepb "github.com/yanet-platform/monalive/gen/manager"
)

// Status retrieves the current status of all reals managed by this service. It
// returns [monalivepb.ServiceStatus] messages representing the status of the
// service and its reals.
func (m *Service) Status() *monalivepb.ServiceStatus {
	// Lock the reals mutex to ensure thread-safe access.
	m.realsMu.Lock()
	defer m.realsMu.Unlock()

	state := m.State()
	alive := 0
	if state.Alive {
		alive = 1
	}

	// Create a slice to hold the statuses of service's reals.
	realStatus := make([]*monalivepb.RealStatus, 0, len(m.reals))
	// Iterate over each real and append it's status to the slice.
	for _, real := range m.reals {
		realStatus = append(realStatus, real.Status())
	}

	// Construct service status based on it's state and reals status slice.
	return &monalivepb.ServiceStatus{
		Vip:                    m.config.VIP.String(),
		Port:                   m.config.VPort.ProtoMarshaller(),
		Protocol:               m.config.Protocol,
		LvsMethod:              m.config.ForwardingMethod,
		QuorumState:            int32(alive),
		AliveWeight:            state.Weight.Uint32(),
		AliveCount:             uint32(state.RealsAlive),
		Transitions:            uint32(state.Transitions),
		Fwmark:                 uint32(m.config.FwMark),
		AnnounceGroup:          &m.config.AnnounceGroup,
		Ipv4OuterSourceNetwork: m.config.IPv4OuterSourceNetwork,
		Ipv6OuterSourceNetwork: m.config.IPv6OuterSourceNetwork,
		Rs:                     realStatus,
	}
}
