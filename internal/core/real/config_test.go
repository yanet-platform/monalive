package real

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yanet-platform/monalive/internal/core/checker"
)

// createRealWithCheckers creates a real config with default checker.
func createRealWithChecker() *Config {
	cfg := DefaultConfig()
	checkerConfig := checker.DefaultConfig()
	checkerConfig.Type = checker.TCPChecker
	cfg.TCPCheckers = append(cfg.TCPCheckers, checkerConfig)
	return cfg
}

// TestPrepare_IPUnmap checks that IPv4-mapped IPv6 addresses are converted to
// IPv4.
func TestPrepare_IPUnmap(t *testing.T) {
	// Create a configuration with IPv4-mapped IPv6 address.
	cfg := DefaultConfig()
	cfg.IP = netip.MustParseAddr("::ffff:192.168.1.1") // IPv4-mapped IPv6 address
	err := cfg.Prepare()
	require.NoError(t, err)
	// Check that the address was converted to IPv4.
	assert.Equal(t, netip.MustParseAddr("192.168.1.1"), cfg.IP)
}

// TestPrepare_CheckerTypes checks that checker types are set correctly.
func TestPrepare_CheckerTypes(t *testing.T) {
	// Create a configuration with checkers.
	cfg := createRealWithChecker()
	cfg.HTTPCheckers = append(cfg.HTTPCheckers, checker.DefaultConfig())
	cfg.HTTPSCheckers = append(cfg.HTTPSCheckers, checker.DefaultConfig())
	cfg.GRPCCheckers = append(cfg.GRPCCheckers, checker.DefaultConfig())
	err := cfg.Prepare()
	require.NoError(t, err)
	// Check that checker types were set correctly.
	assert.Equal(t, checker.TCPChecker, cfg.TCPCheckers[0].Type)
	assert.Equal(t, checker.HTTPChecker, cfg.HTTPCheckers[0].Type)
	assert.Equal(t, checker.HTTPSChecker, cfg.HTTPSCheckers[0].Type)
	assert.Equal(t, checker.GRPCChecker, cfg.GRPCCheckers[0].Type)
}

// TestPrepare_PropagateSchedulerSettings checks that scheduler settings are
// propagated to checkers.
func TestPrepare_PropagateSchedulerSettings(t *testing.T) {
	// Create a configuration with scheduler settings
	cfg := createRealWithChecker()
	delayLoop := 30.0
	retries := 5
	retryDelay := 2.0
	cfg.DelayLoop = &delayLoop
	cfg.Retries = &retries
	cfg.RetryDelay = &retryDelay
	// Clear settings in checker
	cfg.TCPCheckers[0].DelayLoop = nil
	cfg.TCPCheckers[0].Retries = nil
	cfg.TCPCheckers[0].RetryDelay = nil
	err := cfg.Prepare()
	require.NoError(t, err)
	// Check that settings were propagated to checkers
	assert.Equal(t, delayLoop, *cfg.TCPCheckers[0].DelayLoop)
	assert.Equal(t, retries, *cfg.TCPCheckers[0].Retries)
	assert.Equal(t, retryDelay, *cfg.TCPCheckers[0].RetryDelay)
}

// TestPrepare_PropagateVirtualhost checks that Virtualhost is propagated to
// checkers
func TestPrepare_PropagateVirtualhost(t *testing.T) {
	// Create a configuration with Virtualhost
	cfg := createRealWithChecker()
	virtualhost := "example.com"
	cfg.Virtualhost = &virtualhost
	// Clear Virtualhost in checker
	cfg.TCPCheckers[0].Virtualhost = nil
	err := cfg.Prepare()
	require.NoError(t, err)
	// Check that Virtualhost was propagated to checker
	assert.Equal(t, virtualhost, *cfg.TCPCheckers[0].URL.Virtualhost)
}

// TestPrepare_PropagateWithExistingValues checks that existing values in checkers are not overwritten
func TestPrepare_PropagateWithExistingValues(t *testing.T) {
	// Create a configuration with settings
	cfg := createRealWithChecker()
	realDelayLoop := 30.0
	realRetries := 5
	realRetryDelay := 2.0
	realVirtualhost := "real.example.com"
	cfg.DelayLoop = &realDelayLoop
	cfg.Retries = &realRetries
	cfg.RetryDelay = &realRetryDelay
	cfg.Virtualhost = &realVirtualhost
	// Set custom values for checker
	checkerDelayLoop := 15.0
	checkerRetries := 3
	checkerRetryDelay := 1.0
	checkerVirtualhost := "checker.example.com"
	cfg.TCPCheckers[0].DelayLoop = &checkerDelayLoop
	cfg.TCPCheckers[0].Retries = &checkerRetries
	cfg.TCPCheckers[0].RetryDelay = &checkerRetryDelay
	cfg.TCPCheckers[0].Virtualhost = &checkerVirtualhost
	err := cfg.Prepare()
	require.NoError(t, err)
	// Check that checker values were not overwritten
	assert.Equal(t, checkerDelayLoop, *cfg.TCPCheckers[0].DelayLoop)
	assert.Equal(t, checkerRetries, *cfg.TCPCheckers[0].Retries)
	assert.Equal(t, checkerRetryDelay, *cfg.TCPCheckers[0].RetryDelay)
	assert.Equal(t, checkerVirtualhost, *cfg.TCPCheckers[0].Virtualhost)
}
