package real

import (
	"encoding/json"
	"net/netip"
	"slices"
	"strconv"

	"monalive/internal/core/checker"
	"monalive/internal/scheduler"
	"monalive/internal/types/key"
	"monalive/internal/types/port"
	"monalive/internal/types/weight"
	"monalive/internal/utils/coalescer"
)

// Scheduler is a type alias for [scheduler.Config] to provide more informative naming
// for the embedded field in the Config structure.
type Scheduler = scheduler.Config

// Config represents the configuration for a real, including network details,
// scheduling, forwarding methods, and checkers configurations.
type Config struct {
	// IP address of the real server.
	IP netip.Addr `keepalive_pos:"0"`
	// Port of the real server (ommited for L3 balancer service).
	Port port.Port `keepalive_pos:"1"`
	// Weight for of the real.
	Weight weight.Weight `keepalive:"weight"`
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
	TCPCheckers   []*checker.Config `keepalive:"TCP_CHECK"`
	HTTPCheckers  []*checker.Config `keepalive:"HTTP_GET"`
	HTTPSCheckers []*checker.Config `keepalive:"SSL_GET"`
	GRPCCheckers  []*checker.Config `keepalive:"GRPC_CHECK"`
}

// Key returns a [key.Real] struct that uniquely identifies the real by its IP
// address and port.
func (m *Config) Key() key.Real {
	return key.Real{
		Addr: m.IP,
		Port: m.Port,
	}
}

// Default sets the default values for the real configuration.
func (m *Config) Default() {
	m.Weight = 1
}

// Prepare processes the configuration by validating it, unmapping IP addresses,
// and setting the types for each checker.
func (m *Config) Prepare() error {
	// Convert the IP address to its canonical form.
	m.IP = m.IP.Unmap()

	// Set the checker types.
	for _, cfg := range m.TCPCheckers {
		cfg.Type = checker.TCPChecker
	}
	for _, cfg := range m.HTTPCheckers {
		cfg.Type = checker.HTTPChecker
	}
	for _, cfg := range m.HTTPSCheckers {
		cfg.Type = checker.HTTPSChecker
	}
	for _, cfg := range m.GRPCCheckers {
		cfg.Type = checker.GRPCChecker
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
			return err
		}
	}

	return nil
}

// MarshalJSON implements json.Marshaler interface.  This custom marshaller
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

// propagate propagates common configuration from the Config to each checker,
// including scheduling settings and the virtual host.
func (m *Config) propagate(checkers []*checker.Config) {
	for _, checker := range checkers {
		checker.Scheduler.DelayLoop = coalescer.Coalesce(checker.Scheduler.DelayLoop, m.Scheduler.DelayLoop)
		checker.Scheduler.Retries = coalescer.Coalesce(checker.Scheduler.Retries, m.Scheduler.Retries)
		checker.Scheduler.RetryDelay = coalescer.Coalesce(checker.Scheduler.RetryDelay, m.Scheduler.RetryDelay)
		checker.URL.Virtualhost = coalescer.Coalesce(checker.Virtualhost, m.Virtualhost)
	}
}
