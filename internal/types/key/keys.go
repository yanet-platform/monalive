package key

import (
	"net/netip"

	"github.com/yanet-platform/monalive/internal/monitoring/metrics"
	"github.com/yanet-platform/monalive/internal/types/port"
)

type Service struct {
	Addr  netip.Addr
	Port  port.Port
	Proto string
}

func (m Service) Labels() metrics.Labels {
	return metrics.Labels{
		"vip":   m.Addr.String(),
		"vport": m.Port.String(),
		"proto": m.Proto,
	}
}

func (m Service) LabelNames() []string {
	return []string{"vip", "vport", "proto"}
}

func (m Service) Prefix() netip.Prefix {
	return netip.PrefixFrom(m.Addr, m.Addr.BitLen())
}

type Real struct {
	Addr netip.Addr
	Port port.Port
}

func (m Real) Labels() metrics.Labels {
	return metrics.Labels{
		"ip":   m.Addr.String(),
		"port": m.Port.String(),
	}
}

func (m Real) LabelNames() []string {
	return []string{"ip", "port"}
}

type Balancer struct {
	Service Service
	Real    Real
}
