package service

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yanet-platform/monalive/internal/core/real"
)

func defaultServiceConfig() *Config {
	cfg := DefaultConfig()
	cfg.Reals = append(cfg.Reals, real.DefaultConfig())
	return cfg
}

// TestPrepare_IPUnmap checks that IPv4-mapped IPv6 addresses are converted to
// IPv4.
func TestPrepare_IPUnmap(t *testing.T) {
	// Create a configuration with IPv4-mapped IPv6 address.
	cfg := defaultServiceConfig()
	cfg.VIP = netip.MustParseAddr("::ffff:192.168.1.1") // IPv4-mapped IPv6 address
	err := cfg.Prepare()
	require.NoError(t, err)
	// Check that the address was converted to IPv4.
	assert.Equal(t, netip.MustParseAddr("192.168.1.1"), cfg.VIP)
}

// TestPrepare_ProtocolUppercase checks that protocol is converted to uppercase.
func TestPrepare_ProtocolUppercase(t *testing.T) {
	// Create a configuration with lowercase protocol.
	cfg := defaultServiceConfig()
	cfg.Protocol = "tcp"
	err := cfg.Prepare()
	require.NoError(t, err)
	// Check that protocol was converted to uppercase.
	assert.Equal(t, "TCP", cfg.Protocol)
}

// TestPrepare_ForwardingMethodUppercase checks that forwarding method is
// converted to uppercase.
func TestPrepare_ForwardingMethodUppercase(t *testing.T) {
	// Create a configuration with lowercase forwarding method.
	cfg := defaultServiceConfig()
	cfg.ForwardingMethod = "tun"
	err := cfg.Prepare()
	require.NoError(t, err)
	// Check that forwarding method was converted to uppercase.
	assert.Equal(t, "TUN", cfg.ForwardingMethod)
}

// TestPrepare_LVSSchedulerUppercase checks that LVS scheduler is converted to
// lowercase.
func TestPrepare_LVSSchedulerUppercase(t *testing.T) {
	// Create a configuration with uppercase LVS scheduler.
	cfg := defaultServiceConfig()
	cfg.LVSSheduler = "WRR"
	err := cfg.Prepare()
	require.NoError(t, err)
	// Check that LVS scheduler was converted to lowercase.
	assert.Equal(t, "wrr", cfg.LVSSheduler)
}

// TestPrepare_PropagateForwardingMethod tests the behavior of ForwardingMethod
// propagation to reals.
func TestPrepare_PropagateForwardingMethod(t *testing.T) {
	// Create a configuration with empty ForwardingMethod.
	cfg := defaultServiceConfig()
	cfg.Reals[0].ForwardingMethod = "" // empty ForwardingMethod in service
	err := cfg.Prepare()
	require.NoError(t, err)
	// Check that ForwardingMethod was propagated to the real according to
	// current implementation, ForwardingMethod is propagated only if it's empty
	// in the real.
	assert.Equal(t, "TUN", cfg.Reals[0].ForwardingMethod)
}

// TestPrepare_PropagateSchedulerSettings checks that scheduler settings are
// propagated to reals.
func TestPrepare_PropagateSchedulerSettings(t *testing.T) {
	// Create a configuration with scheduler settings.
	cfg := defaultServiceConfig()
	delayLoop := 30.0
	retries := 5
	retryDelay := 2.0
	cfg.DelayLoop = &delayLoop
	cfg.Retries = &retries
	cfg.RetryDelay = &retryDelay
	// Clear settings in real.
	cfg.Reals[0].DelayLoop = nil
	cfg.Reals[0].Retries = nil
	cfg.Reals[0].RetryDelay = nil
	err := cfg.Prepare()
	require.NoError(t, err)
	// Check that settings were propagated to real.
	assert.Equal(t, delayLoop, *cfg.Reals[0].DelayLoop)
	assert.Equal(t, retries, *cfg.Reals[0].Retries)
	assert.Equal(t, retryDelay, *cfg.Reals[0].RetryDelay)
}

// TestPrepare_PropagateVirtualhost checks that Virtualhost is propagated to
// reals.
func TestPrepare_PropagateVirtualhost(t *testing.T) {
	// Create a configuration with Virtualhost
	cfg := defaultServiceConfig()
	virtualhost := "example.com"
	cfg.Virtualhost = &virtualhost
	cfg.Reals[0].Virtualhost = nil // clear Virtualhost in real
	err := cfg.Prepare()
	require.NoError(t, err)
	// Check that Virtualhost was propagated to real.
	assert.Equal(t, virtualhost, *cfg.Reals[0].Virtualhost)
}

// TestPrepare_PropagateWithExistingValues checks that existing values in reals
// are not overwritten.
func TestPrepare_PropagateWithExistingValues(t *testing.T) {
	// Create a configuration with settings.
	cfg := defaultServiceConfig()
	serviceDelayLoop := 30.0
	serviceRetries := 5
	serviceRetryDelay := 2.0
	serviceVirtualhost := "service.example.com"
	cfg.DelayLoop = &serviceDelayLoop
	cfg.Retries = &serviceRetries
	cfg.RetryDelay = &serviceRetryDelay
	cfg.Virtualhost = &serviceVirtualhost
	// Set custom values for real.
	realDelayLoop := 15.0
	realRetries := 3
	realRetryDelay := 1.0
	realVirtualhost := "real.example.com"
	cfg.Reals[0].DelayLoop = &realDelayLoop
	cfg.Reals[0].Retries = &realRetries
	cfg.Reals[0].RetryDelay = &realRetryDelay
	cfg.Reals[0].Virtualhost = &realVirtualhost
	err := cfg.Prepare()
	require.NoError(t, err)
	// Check that real values were not overwritten.
	assert.Equal(t, realDelayLoop, *cfg.Reals[0].DelayLoop)
	assert.Equal(t, realRetries, *cfg.Reals[0].Retries)
	assert.Equal(t, realRetryDelay, *cfg.Reals[0].RetryDelay)
	assert.Equal(t, realVirtualhost, *cfg.Reals[0].Virtualhost)
}

// TestPrepare_ComplexConfig tests Prepare() on a more complex configuration
// with multiple reals.
func TestPrepare_ComplexConfig(t *testing.T) {
	// Create a configuration with multiple reals.
	cfg := defaultServiceConfig()
	// Add a second real.
	real2 := real.DefaultConfig()
	real2.IP = netip.MustParseAddr("::ffff:10.0.0.2") // IPv4-mapped IPv6
	// Set empty ForwardingMethod so it propagates from the service.
	real2.ForwardingMethod = ""
	cfg.Reals = append(cfg.Reals, real2)
	// Configure the service
	cfg.VIP = netip.MustParseAddr("::ffff:192.168.1.1") // IPv4-mapped IPv6
	cfg.Protocol = "udp"
	err := cfg.Prepare()
	require.NoError(t, err)
	// Check IP address conversion.
	assert.Equal(t, netip.MustParseAddr("192.168.1.1"), cfg.VIP)
	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), cfg.Reals[1].IP)
	// Check protocol conversion.
	assert.Equal(t, "UDP", cfg.Protocol)
	// Check settings propagation to the first real.
	assert.Equal(t, "TUN", cfg.Reals[0].ForwardingMethod)
	// Check that DelayLoop, Retries, RetryDelay and Virtualhost values are not
	// nil.
	assert.NotNil(t, cfg.Reals[0].DelayLoop)
	assert.NotNil(t, cfg.Reals[0].Retries)
	assert.NotNil(t, cfg.Reals[0].RetryDelay)
	assert.NotNil(t, cfg.Reals[0].Virtualhost)
	// Check settings propagation to the second real.
	assert.Equal(t, "TUN", cfg.Reals[1].ForwardingMethod)
	// Check that DelayLoop, Retries, RetryDelay and Virtualhost values are not
	// nil.
	assert.NotNil(t, cfg.Reals[1].DelayLoop)
	assert.NotNil(t, cfg.Reals[1].Retries)
	assert.NotNil(t, cfg.Reals[1].RetryDelay)
	assert.NotNil(t, cfg.Reals[1].Virtualhost)
}
