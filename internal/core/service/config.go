package service

import (
	"encoding/json"
	"net/netip"
	"strings"

	"github.com/yanet-platform/monalive/internal/core/real"
	"github.com/yanet-platform/monalive/internal/types/key"
	"github.com/yanet-platform/monalive/internal/utils/exp"
	"github.com/yanet-platform/monalive/pkg/keepalived"
)

type Config keepalived.Service

// Key returns a [key.Service] struct that uniquely identifies the service by
// its virtual IP, port, and protocol.
func (m Config) Key() key.Service {
	return key.Service{Addr: m.VIP, Port: m.VPort, Proto: m.Protocol}
}

// MarshalJSON serializes the service configuration to JSON format, applying
// specific rules for struct fields.
func (m Config) MarshalJSON() ([]byte, error) {
	reals := make([]*real.Config, len(m.Reals))
	for i, r := range m.Reals {
		reals[i] = (*real.Config)(r)
	}
	return json.Marshal(
		struct {
			VIP                    netip.Addr     `json:"vip"`
			VPort                  string         `json:"vport,omitempty"`
			Protocol               string         `json:"proto"`
			Scheduler              string         `json:"scheduler"`
			OnePacketScheduler     bool           `json:"ops"`
			ForwardingMethod       string         `json:"lvs_method"`
			Reals                  []*real.Config `json:"reals"`
			Version                *string        `json:"version,omitempty"`
			IPv4OuterSourceNetwork string         `json:"ipv4_outer_source_network,omitempty"`
			IPv6OuterSourceNetwork string         `json:"ipv6_outer_source_network,omitempty"`
		}{
			VIP:                    m.VIP,
			VPort:                  m.VPort.String(),
			Protocol:               strings.ToLower(m.Protocol),
			Scheduler:              convertScheduler(m.LVSSheduler, m.OnePacketScheduler),
			OnePacketScheduler:     m.OnePacketScheduler,
			ForwardingMethod:       m.ForwardingMethod,
			Reals:                  reals,
			Version:                m.Version,
			IPv4OuterSourceNetwork: m.IPv4OuterSourceNetwork,
			IPv6OuterSourceNetwork: m.IPv6OuterSourceNetwork,
		},
	)
}

// convertScheduler maps certain scheduler types to others based on conditions.
// This function is used temporarily to work around limitations in the current
// scheduler support.
func convertScheduler(scheduler string, ops bool) string {
	// Temporary mh->wrr mapping until YANET supports mh (upd: guess it won't)
	// scheduler.
	if scheduler == "mh" {
		if newSched := exp.MHReplaceValue(); newSched != "" {
			return newSched
		}
		return "wrr"
	}

	// No connections = no wlc
	if ops && scheduler == "wlc" {
		return "wrr"
	}
	return scheduler
}
