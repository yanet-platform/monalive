package checker

import (
	"net/netip"
	"time"

	"github.com/yanet-platform/monalive/pkg/keepalived"
	port "github.com/yanet-platform/monalive/pkg/optport"
)

type Config keepalived.Checker

// Key represents the full configuration of a checker, with pointers replaced by
// values. This structure is used to uniquely identify a checker configuration.
type Key struct {
	ty keepalived.CheckerType

	connectIP      netip.Addr
	connectPort    port.Port
	bindIP         netip.Addr
	connectTimeout float64
	checkTimeout   float64
	fwMark         int

	path        string
	statusCode  int
	digest      string
	virtualhost string

	dynamicWeight       bool
	dynamicWeightHeader bool
	dynamicWeightCoeff  uint

	delayLoop  time.Duration
	retries    int
	retryDelay time.Duration
}

// Key returns a Key representation of the Config, which includes all necessary
// fields to uniquely identify the checker configuration.
//
// TODO: seems that not all of the checker fields must be treated as its key.
// Some of the parameters can be updated at the runtime.
func (m *Config) Key() Key {
	var virtualhost string
	if m.Virtualhost != nil {
		virtualhost = *m.Virtualhost
	}

	return Key{
		ty: m.Type,

		connectIP:      m.ConnectIP,
		connectPort:    m.ConnectPort,
		bindIP:         m.BindIP,
		connectTimeout: m.CheckTimeout,
		checkTimeout:   m.CheckTimeout,
		fwMark:         m.FWMark,

		path:        m.Path,
		statusCode:  m.StatusCode,
		digest:      m.Digest,
		virtualhost: virtualhost,

		dynamicWeight:       m.DynamicWeight,
		dynamicWeightHeader: m.DynamicWeightHeader,
		dynamicWeightCoeff:  m.DynamicWeightCoeff,

		delayLoop:  m.GetDelayLoop(),
		retries:    m.GetRetries(),
		retryDelay: m.GetRetryDelay(),
	}
}
