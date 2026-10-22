package real

import (
	"encoding/json"
	"net/netip"
	"strconv"

	"github.com/yanet-platform/monalive/internal/types/key"
	"github.com/yanet-platform/monalive/pkg/keepalived"
)

type Config keepalived.Real

// Key returns a [key.Real] struct that uniquely identifies the real by its IP
// address and port.
func (m Config) Key() key.Real {
	return key.Real{Addr: m.IP, Port: m.Port}
}

// MarshalJSON implements json.Marshaler interface.
//
// This custom marshaller
// converts Port and Weight to strings and includes only necessary fields in the
// JSON output.
func (m Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		struct {
			IP     netip.Addr `json:"ip"`
			Port   string     `json:"port,omitempty"`
			Weight string     `json:"weight"`
		}{
			IP:     m.IP,
			Port:   m.Port.String(),
			Weight: strconv.Itoa(int(m.Weight)),
		},
	)
}
