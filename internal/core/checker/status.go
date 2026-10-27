package checker

import (
	"net/netip"
	"time"

	"github.com/yanet-platform/monalive/pkg/optport"
)

// Status is a snapshot of a checker configuration and current state.
type Status struct {
	Type                string
	ConnectIP           netip.Addr
	ConnectPort         optport.Port
	BindIP              netip.Addr
	ConnectTimeout      time.Duration
	CheckTimeout        time.Duration
	FWMark              int
	Path                string
	StatusCode          int
	Digest              string
	Virtualhost         *string
	DynamicWeight       bool
	DynamicWeightHeader bool
	DynamicWeightCoeff  uint
	DelayLoop           time.Duration
	Retries             int
	RetryDelay          time.Duration
	Alive               bool
	FailedAttempts      int
	LastCheckTS         time.Time
}

// Status retrieves the current checker snapshot, owning its optional values.
func (m *Checker) Status() Status {
	state := m.State()

	var virtualhost *string
	if m.config.Virtualhost != nil {
		value := *m.config.Virtualhost
		virtualhost = &value
	}

	return Status{
		Type:                m.config.Type.String(),
		ConnectIP:           m.config.ConnectIP,
		ConnectPort:         m.config.ConnectPort,
		BindIP:              m.config.BindIP,
		ConnectTimeout:      m.config.GetConnectTimeout(),
		CheckTimeout:        m.config.GetCheckTimeout(),
		FWMark:              m.config.FWMark,
		Path:                m.config.Path,
		StatusCode:          m.config.StatusCode,
		Digest:              m.config.Digest,
		Virtualhost:         virtualhost,
		DynamicWeight:       m.config.DynamicWeight,
		DynamicWeightHeader: m.config.DynamicWeightHeader,
		DynamicWeightCoeff:  m.config.DynamicWeightCoeff,
		DelayLoop:           m.config.GetDelayLoop(),
		Retries:             m.config.GetRetries(),
		RetryDelay:          m.config.GetRetryDelay(),
		Alive:               state.Alive,
		FailedAttempts:      state.FailedAttempts,
		LastCheckTS:         state.Timestamp,
	}
}
