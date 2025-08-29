package key

import (
	"net/netip"

	"github.com/yanet-platform/monalive/internal/types/port"
)

type Service struct {
	Addr  netip.Addr
	Port  port.Port
	Proto string
}

func (m Service) Prefix() netip.Prefix {
	return netip.PrefixFrom(m.Addr, m.Addr.BitLen())
}

type Real struct {
	Addr netip.Addr
	Port port.Port
}

type Balancer struct {
	Service Service
	Real    Real
}
