package keepalived

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/yanet-platform/monalive/pkg/coalescer"
	"github.com/yanet-platform/monalive/pkg/keepalived/internal/utils"
	"github.com/yanet-platform/monalive/pkg/optport"
)

var ErrInvalidQuorumScript = errors.New("invalid quorum script")

// VSKey is an identifier of a service that contains a virtual IP address,
// protocol and optional port.
type VSKey struct {
	// Virtual IP address of the service.
	VIP netip.Addr `keepalive_pos:"0"`
	// Virtual port of the service (ommited for L3 balancer service).
	VPort optport.Port `keepalive_pos:"1"`
	// Protocol used by the service (e.g., TCP, UDP).
	Protocol string `keepalive:"protocol"`
}

// Config represents the configuration for a service, including network details,
// scheduling, forwarding methods, quorum settings, and real server
// configurations.
type Service struct {
	// VSKey is an embedded identifier of a service that contains a virtual IP
	// address, protocol and optional port.
	VSKey `keepalive_nested:"vs_key"`
	// LVS (Linux Virtual Server) scheduler type.
	LVSSheduler string `keepalive:"lvs_sched"`
	// Forwarding method (TUN, GRE) to send health checks to the service.
	ForwardingMethod string `keepalive:"lvs_method"`
	// Quorum is the required weight for service to be enabled.
	Quorum int `keepalive:"quorum" default:"1"`
	// Hysteresis setting for quorum calculations.
	Hysteresis int `keepalive:"hysteresis"`
	// Script executed (no) when quorum is achieved.
	QuorumUp string `keepalive:"quorum_up"`
	// Script executed (no) when quorum is lost.
	QuorumDown string `keepalive:"quorum_down"`
	// Announce group for BGP prefix announcement.
	AnnounceGroup string `keepalive:"announce_group"`
	// Optional virtual host for the service.
	Virtualhost *string `keepalive:"virtualhost"`
	// Firewall mark for packet filtering.
	FwMark int `keepalive:"fwmark"`
	// Enable one-packet-scheduler (OPS) for UDP balancing.
	OnePacketScheduler bool `keepalive:"ops"`
	// Outer source network for IPv4.
	IPv4OuterSourceNetwork string `keepalive:"ipv4_outer_source_network"`
	// Outer source network for IPv6.
	IPv6OuterSourceNetwork string `keepalive:"ipv6_outer_source_network"`
	// Optional version identifier of the service config.
	Version *string `keepalive:"version"`

	// Embedded scheduler configuration.
	Scheduler `keepalive_nested:"scheduler"`

	// List of real server configurations.
	Reals []*Real `keepalive:"real_server"`
}

// Default sets the default values for the service configuration.
func (m *Service) Default() {
	m.VPort = optport.Omitted
	m.ForwardingMethod = "TUN"
	m.Quorum = 1
	m.Scheduler.Default()
}

// Equal returns true if the service configuration is equal to the other one.
func (m *Service) Equal(other *Service) bool {
	return m.VIP == other.VIP &&
		m.VPort == other.VPort &&
		m.Protocol == other.Protocol &&
		m.LVSSheduler == other.LVSSheduler &&
		m.ForwardingMethod == other.ForwardingMethod &&
		m.Quorum == other.Quorum &&
		m.Hysteresis == other.Hysteresis &&
		m.QuorumUp == other.QuorumUp &&
		m.QuorumDown == other.QuorumDown &&
		m.AnnounceGroup == other.AnnounceGroup &&
		utils.ComparePtr(m.Virtualhost, other.Virtualhost) &&
		m.FwMark == other.FwMark &&
		m.OnePacketScheduler == other.OnePacketScheduler &&
		m.IPv4OuterSourceNetwork == other.IPv4OuterSourceNetwork &&
		m.IPv6OuterSourceNetwork == other.IPv6OuterSourceNetwork &&
		utils.ComparePtr(m.Version, other.Version) &&
		m.Scheduler.Equal(other.Scheduler)
}

// Prepare validates and processes the service configuration, unmaps IP
// addresses, and propagates settings to the real server configurations.
func (m *Service) Prepare() error {
	// Convert the IP address to its canonical form.
	m.VIP = m.VIP.Unmap()
	// Ensure the protocol is uppercase.
	m.Protocol = strings.ToUpper(m.Protocol)
	// Ensure the forwarding method is uppercase.
	m.ForwardingMethod = strings.ToUpper(m.ForwardingMethod)
	// Ensure the LVS Sheduler is lowercase.
	m.LVSSheduler = strings.ToLower(m.LVSSheduler)

	// Validate the configuration.
	if err := m.validate(); err != nil {
		return fmt.Errorf("failed to validate service: %w", err)
	}

	// Propagate necessary settings to real server configurations.
	m.propagate()

	// Prepare each real server configuration.
	for _, real := range m.Reals {
		if err := real.Prepare(); err != nil {
			return fmt.Errorf("failed to prepare real: %w", err)
		}
	}

	return nil
}

// propagate applies necessary configuration values to all reals in the config.
// It ensures that each real server inherits the relevant settings from the
// service.
func (m *Service) propagate() {
	for _, real := range m.Reals {
		if real.ForwardingMethod == "" {
			real.ForwardingMethod = m.ForwardingMethod
		}
		real.DelayLoop = coalescer.Coalesce(real.DelayLoop, m.DelayLoop)
		real.Retries = coalescer.Coalesce(real.Retries, m.Retries)
		real.RetryDelay = coalescer.Coalesce(real.RetryDelay, m.RetryDelay)
		real.Virtualhost = coalescer.Coalesce(real.Virtualhost, m.Virtualhost)
	}
}

// validate checks the configuration for any errors or missing values and
// ensures that critical fields like the announce group are set correctly.
func (m *Service) validate() error {
	if m.AnnounceGroup != "" {
		return nil
	}

	announceGroup, err := m.announceGroupFromQuorumScript()
	if err != nil {
		return fmt.Errorf("failed to get announce group: %w", err)
	}
	m.AnnounceGroup = announceGroup

	return nil
}

// announceGroupFromQuorumScript extracts the announce group from the quorum
// script. If the script is invalid, an error is returned.
func (m *Service) announceGroupFromQuorumScript() (string, error) {
	if m.QuorumUp == "" {
		// The quorum script can be empty. If it is empty, then the service
		// doesn't affect the announce of the host prefix.
		return "", nil
	}

	script := m.QuorumUp
	quorumFields := strings.Fields(script)
	if len(quorumFields) != 3 {
		return "", fmt.Errorf("%w: %s", ErrInvalidQuorumScript, script)
	}
	if quorumFields[0] != "/etc/keepalived/quorum-handler2.sh" {
		return "", fmt.Errorf("%w: incorrect script: %s", ErrInvalidQuorumScript, script)
	}

	fields := strings.Split(quorumFields[2], ",")
	if len(fields) < 2 {
		return "", fmt.Errorf("%w: not enough args: %s", ErrInvalidQuorumScript, script)
	}

	group := fields[len(fields)-2]

	return group, nil
}

// DefaultService return default service configuration. Used for testing
// purpuses only.
func DefaultService() *Service {
	virtualhost := "virtualhost"
	version := "1.0.0"

	var schedConfig Scheduler
	schedConfig.Default()

	return &Service{
		VSKey: VSKey{
			VIP:      netip.MustParseAddr("10.0.0.0"),
			VPort:    80,
			Protocol: "tcp",
		},
		LVSSheduler:            "wrr",
		ForwardingMethod:       "tun",
		Quorum:                 10,
		Hysteresis:             10,
		QuorumUp:               "",
		QuorumDown:             "",
		AnnounceGroup:          "",
		Virtualhost:            &virtualhost,
		FwMark:                 12345,
		OnePacketScheduler:     false,
		IPv4OuterSourceNetwork: "10.0.0.0/8",
		IPv6OuterSourceNetwork: "fd00::/8",
		Version:                &version,
		Scheduler:              schedConfig,
		Reals:                  nil,
	}
}
