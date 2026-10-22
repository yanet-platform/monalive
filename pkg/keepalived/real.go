package keepalived

import (
	"fmt"
	"net/netip"
	"slices"

	"github.com/yanet-platform/monalive/pkg/coalescer"
	"github.com/yanet-platform/monalive/pkg/keepalived/internal/utils"
	"github.com/yanet-platform/monalive/pkg/optport"
	"github.com/yanet-platform/monalive/pkg/weight"
)

// RSKey is an identifier of a real server that contains a real IP address and
// optional port.
type RSKey struct {
	// IP address of the real server.
	IP netip.Addr `keepalive_pos:"0"`
	// Port of the real server (ommited for L3 balancer service).
	Port optport.Port `keepalive_pos:"1"`
}

// Config represents the configuration for a real, including network details,
// scheduling, forwarding methods, and checkers configurations.
type Real struct {
	// RSKey is an embedded identifier of a real server that contains a real IP
	// address and optional port.
	RSKey `keepalive_nested:"rs_key"`
	// Weight for of the real.
	Weight weight.Weight `keepalive:"weight" default:"1"`
	// Inhibit on failure flag.
	//
	// If checker reports a failure and this option is set, then instead of
	// disabling real, we keep it enabled, but set its weight to zero.
	InhibitOnFailure bool `keepalive:"inhibit_on_failure"`
	// Optional virtual host.
	Virtualhost *string `keepalive:"virtualhost"` // optional
	// Forwarding method (TUN, GRE) to send health checks to the service.
	ForwardingMethod string `keepalive:"lvs_method"` // optional

	// Embedded scheduler configuration.
	Scheduler `keepalive_nested:"scheduler"`

	// List of checker configurations separated by their types.
	TCPCheckers   []*Checker `keepalive:"TCP_CHECK"`
	HTTPCheckers  []*Checker `keepalive:"HTTP_GET"`
	HTTPSCheckers []*Checker `keepalive:"SSL_GET"`
	GRPCCheckers  []*Checker `keepalive:"GRPC_CHECK"`
}

// Checkers returns slice of all checker configurations of the real.
func (m *Real) Checkers() []*Checker {
	return slices.Concat(
		m.TCPCheckers,
		m.HTTPCheckers,
		m.HTTPSCheckers,
		m.GRPCCheckers,
	)
}

// Default sets the default values for the real configuration.
func (m *Real) Default() {
	m.Port = optport.Omitted
	m.Weight = 1
}

// Equal returns true if the real configuration is equal to the other one. It
// ignores underlying checker configurations.
func (m *Real) Equal(other *Real) bool {
	return m.IP == other.IP &&
		m.Port == other.Port &&
		m.Weight == other.Weight &&
		m.InhibitOnFailure == other.InhibitOnFailure &&
		utils.ComparePtr(m.Virtualhost, other.Virtualhost) &&
		m.ForwardingMethod == other.ForwardingMethod &&
		m.Scheduler.Equal(other.Scheduler)
}

// Prepare processes the configuration by validating it, unmapping IP addresses,
// and setting the types for each checker.
func (m *Real) Prepare() error {
	// Convert the IP address to its canonical form.
	m.IP = m.IP.Unmap()

	// Set the checker types.
	for _, cfg := range m.TCPCheckers {
		cfg.Type = TCPChecker
	}
	for _, cfg := range m.HTTPCheckers {
		cfg.Type = HTTPChecker
	}
	for _, cfg := range m.HTTPSCheckers {
		cfg.Type = HTTPSChecker
	}
	for _, cfg := range m.GRPCCheckers {
		cfg.Type = GRPCChecker
	}

	// Combine all checkers into a single slice.
	checkers := slices.Concat(
		m.TCPCheckers,
		m.HTTPCheckers,
		m.HTTPSCheckers,
		m.GRPCCheckers,
	)
	// Propagate common configuration to checkers.
	m.propagate(checkers)

	// Prepare each checker configuration.
	for _, checker := range checkers {
		if err := checker.Prepare(); err != nil {
			return fmt.Errorf("failed to prepare checker: %w", err)
		}
	}

	return nil
}

// propagate propagates common configuration from the Config to each checker,
// including scheduling settings and the virtual host.
func (m *Real) propagate(checkers []*Checker) {
	for _, checker := range checkers {
		checker.DelayLoop = coalescer.Coalesce(checker.DelayLoop, m.DelayLoop)
		checker.Retries = coalescer.Coalesce(checker.Retries, m.Retries)
		checker.RetryDelay = coalescer.Coalesce(checker.RetryDelay, m.RetryDelay)
		checker.URL.Virtualhost = coalescer.Coalesce(checker.Virtualhost, m.Virtualhost)
	}
}

// DefaultReal return default real configuration. Used for testing purpuses
// only.
func DefaultReal() *Real {
	virtualhost := "virtualhost"

	var schedConfig Scheduler
	schedConfig.Default()

	return &Real{
		RSKey: RSKey{
			IP:   netip.MustParseAddr("127.0.0.1"),
			Port: 80,
		},
		Weight:           1,
		InhibitOnFailure: false,
		Virtualhost:      &virtualhost,
		ForwardingMethod: "TUN",
		Scheduler:        schedConfig,
		TCPCheckers:      nil,
		HTTPCheckers:     nil,
		HTTPSCheckers:    nil,
		GRPCCheckers:     nil,
	}
}
