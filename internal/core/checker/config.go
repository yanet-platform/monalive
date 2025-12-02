package checker

import (
	"net/netip"
	"time"

	"github.com/yanet-platform/monalive/internal/core/checker/check"
	"github.com/yanet-platform/monalive/internal/scheduler"
	"github.com/yanet-platform/monalive/internal/types/port"
)

// Type represents the type of the checker (TCP, HTTP, etc.).
type Type int

const (
	TCPChecker Type = iota + 1
	HTTPChecker
	HTTPSChecker
	GRPCChecker
)

func (m Type) String() string {
	switch m {
	case TCPChecker:
		return "TCP"
	case HTTPChecker:
		return "HTTP"
	case HTTPSChecker:
		return "HTTPS"
	case GRPCChecker:
		return "GRPC"
	default:
		return "unknown"
	}
}

type (
	// Scheduler is a type alias for [scheduler.Config] to provide more
	// informative naming for the embedded field in the Config structure.
	Scheduler = scheduler.Config

	// CheckConfig is a type alias for [check.Config] to provide more
	// informative naming for the embedded field in the Config structure.
	CheckConfig = check.Config
)

// Key represents the full configuration of a checker, with pointers replaced by
// values. This structure is used to uniquely identify a checker configuration.
type Key struct {
	ty Type

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

// Config holds the configuration for a checker, including its type and various
// settings.
type Config struct {
	Type        Type
	CheckConfig `keepalive_nested:"check"`
	Scheduler   `keepalive_nested:"scheduler"`
}

// Default initializes the Config with default values.
// This method is a placeholder for any default initialization logic.
func (m *Config) Default() {
	// Just to override embedded one.
}

// Prepare processes the configuration by unmapping IP addresses.
func (m *Config) Prepare() error {
	m.BindIP = m.BindIP.Unmap()
	m.ConnectIP = m.ConnectIP.Unmap()

	return nil
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

// DefaultConfig return default checker configuration.
// Used for testing purpuses only.
func DefaultConfig() *Config {
	var schedConfig Scheduler
	schedConfig.Default()
	return &Config{
		CheckConfig: check.DefaultConfig(),
		Scheduler:   schedConfig,
	}
}
