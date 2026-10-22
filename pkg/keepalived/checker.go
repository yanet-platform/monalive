package keepalived

import (
	"net/netip"
	"time"

	"github.com/yanet-platform/monalive/pkg/keepalived/internal/utils"
	"github.com/yanet-platform/monalive/pkg/optport"
)

// CheckerType represents the type of the checker (TCP, HTTP, etc.).
type CheckerType int

const (
	TCPChecker CheckerType = iota + 1
	HTTPChecker
	HTTPSChecker
	GRPCChecker
)

func (m CheckerType) String() string {
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

// Config holds the configuration for a checker, including its type and various
// settings.
type Checker struct {
	Type      CheckerType
	Check     `keepalive_nested:"check"`
	Scheduler `keepalive_nested:"scheduler"`
}

// Default initializes the Config with default values.
// This method is a placeholder for any default initialization logic.
func (m *Checker) Default() {
	// Just to override embedded one.
}

// Equal returns true if the checker configuration is equal to the other one.
func (m *Checker) Equal(other *Checker) bool {
	return m.Type == other.Type &&
		m.Check.Equal(other.Check) &&
		m.Scheduler.Equal(other.Scheduler)
}

// Prepare processes the configuration by unmapping IP addresses.
func (m *Checker) Prepare() error {
	m.BindIP = m.BindIP.Unmap()
	m.ConnectIP = m.ConnectIP.Unmap()

	return nil
}

// DefaultConfig return default checker configuration.
// Used for testing purpuses only.
func DefaultCheckerConfig() *Checker {
	var schedConfig Scheduler
	schedConfig.Default()

	return &Checker{
		Check:     DefaultCheckConfig(),
		Scheduler: schedConfig,
	}
}

// Config represents the full configuration for a health check. It includes URL
// settings, network settings, and weight control settings.
type Check struct {
	URL           `keepalive:"url"`
	Net           `keepalive_nested:"net"`
	WeightControl `keepalive_nested:"weight_control"`
}

// Equal returns true if the checker configuration is equal to the other one.
func (m Check) Equal(other Check) bool {
	return m.URL.Equal(other.URL) &&
		m.Net.Equal(other.Net) &&
		m.WeightControl.Equal(other.WeightControl)
}

// URL contains settings specific to the URL check, such as the URL path,
// expected status code, digest for response validation, and optional virtual
// host.
type URL struct {
	// Path is the URL path to be used for the health check.
	Path string `keepalive:"path" json:"path"`
	// StatusCode is the expected HTTP status code for a successful check.
	StatusCode int `keepalive:"status_code" json:"status"`
	// Digest is used for validating the content of the response.
	Digest string `keepalive:"digest" json:"digest"`
	// Virtualhost is an optional field specifying the virtual host for
	// HTTP/HTTPS checks. It allows the check to target different services or
	// configurations by hostname.
	//
	// Also its can be used as gRPC Service in gRPC checks.
	Virtualhost *string `keepalive:"virtualhost" json:"virtualhost"`
}

// Equal returns true if the URL is equal to the other one.
func (m URL) Equal(other URL) bool {
	return m.Path == other.Path &&
		m.StatusCode == other.StatusCode &&
		m.Digest == other.Digest &&
		utils.ComparePtr(m.Virtualhost, other.Virtualhost)
}

// WeightControl holds configuration related to dynamic weight adjustment for
// the check.
type WeightControl struct {
	// DynamicWeight enables or disables dynamic weight adjustment based on the
	// check result.
	DynamicWeight bool `keepalive:"dynamic_weight_enable"`
	// DynamicWeightHeader specifies whether dynamic weighting is based on HTTP
	// headers or body.
	DynamicWeightHeader bool `keepalive:"dynamic_weight_in_header"`
	// DynamicWeightCoeff is the coefficient used to calculate weight
	// adjustments. It's a percentage that determines how much the weight should
	// change based on check results.
	DynamicWeightCoeff uint `keepalive:"dynamic_weight_coefficient"`
}

// Equal returns true if the WeightControl is equal to the other one.
func (m WeightControl) Equal(other WeightControl) bool {
	return m.DynamicWeight == other.DynamicWeight &&
		m.DynamicWeightHeader == other.DynamicWeightHeader &&
		m.DynamicWeightCoeff == other.DynamicWeightCoeff
}

// Net contains network configuration for the health check, including IP
// addresses, ports, timeouts, and firewall mark.
type Net struct {
	// ConnectIP is the IP address used to connect to the service being checked.
	ConnectIP netip.Addr `keepalive:"connect_ip"`
	// ConnectPort is the port used to connect to the service being checked.
	ConnectPort optport.Port `keepalive:"connect_port"`
	// BindIP is the IP address which will be used as local address for the
	// connection.
	BindIP netip.Addr `keepalive:"bindto"`
	// ConnectTimeout is the timeout for establishing a connection to the
	// service. It's specified in seconds.
	ConnectTimeout float64 `keepalive:"connect_timeout"`
	// CheckTimeout is the timeout for the overall check, including waiting for
	// the service's response. It's specified in seconds.
	CheckTimeout float64 `keepalive:"check_timeout"`
	// FWMark is a firewall mark used for packet filtering, if applicable.
	FWMark int `keepalive:"fwmark"`
}

// Equal returns true if the Net is equal to the other one.
func (m Net) Equal(other Net) bool {
	return m.ConnectIP.Compare(other.ConnectIP) == 0 &&
		m.ConnectPort == other.ConnectPort &&
		m.BindIP.Compare(other.BindIP) == 0 &&
		m.ConnectTimeout == other.ConnectTimeout &&
		m.CheckTimeout == other.CheckTimeout &&
		m.FWMark == other.FWMark
}

// GetConnectTimeout converts the connect timeout from seconds to
// [time.Duration].
func (m *Net) GetConnectTimeout() time.Duration {
	return time.Duration(m.ConnectTimeout * float64(time.Second))
}

// GetCheckTimeout converts the check timeout from seconds to time.Duration.
// If CheckTimeout is not set (i.e., zero), it defaults to using the
// ConnectTimeout value.
func (m *Net) GetCheckTimeout() time.Duration {
	// For backwards compatibility: configs without check_timeout use
	// connect_timeout as check_timeout.
	if m.CheckTimeout == 0 {
		return m.GetConnectTimeout()
	}

	return time.Duration(m.CheckTimeout * float64(time.Second))
}

// DefaultConfig return default check configuration.
// Used for testing purpuses only.
func DefaultCheckConfig() Check {
	virtualhost := "virtualhost"
	return Check{
		URL: URL{
			Path:        "/",
			StatusCode:  200,
			Digest:      "abcd",
			Virtualhost: &virtualhost,
		},
		Net: Net{
			ConnectIP:      netip.MustParseAddr("127.0.0.1"),
			ConnectPort:    80,
			BindIP:         netip.MustParseAddr("127.0.0.2"),
			ConnectTimeout: 5,
			CheckTimeout:   10,
			FWMark:         12345,
		},
		WeightControl: WeightControl{
			DynamicWeight:       true,
			DynamicWeightHeader: true,
			DynamicWeightCoeff:  10,
		},
	}
}
