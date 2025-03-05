package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"monalive/internal/core/real"
	"monalive/internal/scheduler"
	"monalive/internal/types/key"
	"monalive/internal/types/port"
	"monalive/internal/utils/coalescer"
)

var ErrInvalidQuorumScript = errors.New("invalid quorum script")

// Scheduler is a type alias for [scheduler.Config] to provide more informative naming
// for the embedded field in the Config structure.
type Scheduler = scheduler.Config

// Config represents the configuration for a service, including network details,
// scheduling, forwarding methods, quorum settings, and real server
// configurations.
type Config struct {
	// Virtual IP address of the service.
	VIP netip.Addr `keepalive_pos:"0"`
	// Virtual port of the service (ommited for L3 balancer service).
	VPort port.Port `keepalive_pos:"1"`
	// Protocol used by the service (e.g., TCP, UDP).
	Protocol string `keepalive:"protocol"`
	// LVS (Linux Virtual Server) scheduler type.
	LVSSheduler string `keepalive:"lvs_sched"`
	// Forwarding method (TUN, GRE) to send health checks to the service.
	ForwardingMethod string `keepalive:"lvs_method"`
	// Quorum is the required weight for service to be enabled.
	Quorum int `keepalive:"quorum"`
	// Hysteresis setting for quorum calculations.
	Hysteresis int `keepalive:"hysteresis"`
	// Script executed (no) when quorum is achieved.
	QuorumUp string `keepalive:"quorum_up"`
	// Script executed (no) when quorum is lost.
	QuorumDown string `keepalive:"quorum_down"`
	// The prefix group to which the service belongs.
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
	Reals []*real.Config `keepalive:"real_server"`
}

// Key returns a [key.Service] struct that uniquely identifies the service by
// its virtual IP, port, and protocol.
func (m *Config) Key() key.Service {
	return key.Service{
		Addr:  m.VIP,
		Port:  m.VPort,
		Proto: m.Protocol,
	}
}

// Default sets the default values for the service configuration.
func (m *Config) Default() {
	m.ForwardingMethod = "TUN"
	m.Quorum = 1
	m.Scheduler.Default()
}

// Prepare validates and processes the service configuration, unmaps IP
// addresses, and propagates settings to the real server configurations.
func (m *Config) Prepare() error {
	// Convert the IP address to its canonical form.
	m.VIP = m.VIP.Unmap()
	// Ensure the protocol is uppercase.
	m.Protocol = strings.ToUpper(m.Protocol)

	// Validate the configuration.
	if err := m.validate(); err != nil {
		return err
	}

	// Propagate necessary settings to real server configurations.
	m.propagate()

	// Prepare each real server configuration.
	for _, real := range m.Reals {
		if err := real.Prepare(); err != nil {
			return err
		}
	}

	return nil
}

// MarshalJSON serializes the service configuration to JSON format, applying
// specific rules for struct fields.
func (m Config) MarshalJSON() ([]byte, error) {
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
			Reals:                  m.Reals,
			Version:                m.Version,
			IPv4OuterSourceNetwork: m.IPv4OuterSourceNetwork,
			IPv6OuterSourceNetwork: m.IPv6OuterSourceNetwork,
		},
	)
}

// propagate applies necessary configuration values to all reals in the config.
// It ensures that each real server inherits the relevant settings from the
// service.
func (m *Config) propagate() {
	for _, real := range m.Reals {
		if m.ForwardingMethod == "" {
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
func (m *Config) validate() (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("service config validation error occured: %w", err)
		}
	}()

	if m.AnnounceGroup == "" {
		m.AnnounceGroup, err = m.announceGroupFromQuorumScript()
		if err != nil {
			return err
		}
	}

	return nil
}

// announceGroupFromQuorumScript extracts the announce group from the quorum
// script. If the script is invalid, an error is returned.
func (m *Config) announceGroupFromQuorumScript() (string, error) {
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

// convertScheduler maps certain scheduler types to others based on conditions.
// This function is used temporarily to work around limitations in the current
// scheduler support.
func convertScheduler(scheduler string, ops bool) string {
	// Temporary mh->wrr mapping until YANET supports mh (upd: guess it won't)
	// scheduler.
	if scheduler == "mh" {
		return "wrr"
	}

	// No connections = no wlc
	if ops && scheduler == "wlc" {
		return "wrr"
	}
	return scheduler
}
