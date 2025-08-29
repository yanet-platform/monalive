package check

import (
	"net/netip"
	"time"

	"github.com/yanet-platform/monalive/internal/types/port"
)

// Config represents the full configuration for a health check. It includes URL
// settings, network settings, and weight control settings.
type Config struct {
	URL           `keepalive:"url"`
	Net           `keepalive_nested:"net"`
	WeightControl `keepalive_nested:"weight_control"`
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

// Net contains network configuration for the health check, including IP
// addresses, ports, timeouts, and firewall mark.
type Net struct {
	// ConnectIP is the IP address used to connect to the service being checked.
	ConnectIP netip.Addr `keepalive:"connect_ip"`
	// ConnectPort is the port used to connect to the service being checked.
	ConnectPort port.Port `keepalive:"connect_port"`
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
