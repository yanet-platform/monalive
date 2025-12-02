package announcer

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yanet-platform/monalive/internal/types/key"
)

// Helper function to create a default service.
func defaultService() key.Service {
	return key.Service{Addr: netip.MustParseAddr("127.0.0.1"), Port: 80, Proto: "TCP"}
}

// Helper function to create a list of default services.
func defaultServices() []key.Service {
	return []key.Service{
		{Addr: netip.MustParseAddr("127.0.0.1"), Port: 80, Proto: "TCP"},          // Service1
		{Addr: netip.MustParseAddr("127.0.0.1"), Port: 443, Proto: "TCP"},         // Service2
		{Addr: netip.MustParseAddr("2001:dead:beef::1"), Port: 80, Proto: "TCP"},  // Service3
		{Addr: netip.MustParseAddr("2001:dead:beef::1"), Port: 443, Proto: "TCP"}, // Service4
	}
}

// TestPrefix_ReloadServices_InitialReload tests the initial loading of services
// into the prefix registry.
func TestPrefix_ReloadServices_InitialReload(t *testing.T) {
	services := defaultServices()
	prefixRegistry := NewPrefixes()
	prefixRegistry.ReloadServices(services)

	// Expected prefixes and their states after the initial reload.
	prefixes := map[netip.Prefix]*prefixState{
		netip.MustParsePrefix("127.0.0.1/32"):          newState(services[:2]),
		netip.MustParsePrefix("2001:dead:beef::1/128"): newState(services[2:]),
	}
	assert.Equal(t, prefixes, prefixRegistry.prefixes)
}

// TestPrefix_EnableDisabledService tests enabling a service that was previously
// disabled.
func TestPrefix_EnableDisabledService(t *testing.T) {
	service := defaultService()
	prefix := service.Prefix()

	prefixRegistry := NewPrefixes()
	prefixRegistry.ReloadServices([]key.Service{service})

	// Enable the service and check that it's added to the active services.
	require.NoError(t, prefixRegistry.UpdateService(service, ServiceEnabled))
	require.Contains(t, prefixRegistry.prefixes, prefix)
	assert.Equal(t, map[key.Service]ServiceStatus{service: ServiceEnabled}, prefixRegistry.prefixes[prefix].services)
	assert.Equal(t, 1, prefixRegistry.prefixes[prefix].active)
}

// TestPrefix_DisableDisabledService tests disabling a service that is already
// disabled.
func TestPrefix_DisableDisabledService(t *testing.T) {
	service := defaultService()
	prefix := service.Prefix()

	prefixRegistry := NewPrefixes()
	prefixRegistry.ReloadServices([]key.Service{service})

	// Disable the service and check that it has no active services.
	require.NoError(t, prefixRegistry.UpdateService(service, ServiceDisabled))
	require.Contains(t, prefixRegistry.prefixes, prefix)
	assert.Equal(t, 0, prefixRegistry.prefixes[prefix].active)
}

// TestPrefix_EnableEnabledService tests enabling a service that is already
// enabled.
func TestPrefix_EnableEnabledService(t *testing.T) {
	service := defaultService()
	prefix := service.Prefix()

	prefixRegistry := NewPrefixes()
	prefixRegistry.ReloadServices([]key.Service{service})

	// Enable the service twice and check that it remains active.
	require.NoError(t, prefixRegistry.UpdateService(service, ServiceEnabled))
	require.NoError(t, prefixRegistry.UpdateService(service, ServiceEnabled))
	require.Contains(t, prefixRegistry.prefixes, prefix)
	require.Contains(t, prefixRegistry.prefixes[prefix].services, service)
	assert.Equal(t, ServiceEnabled, prefixRegistry.prefixes[prefix].services[service])
	assert.Equal(t, 1, prefixRegistry.prefixes[prefix].active)
}

// TestPrefix_DisableEnabledService tests disabling a service that is currently
// enabled.
func TestPrefix_DisableEnabledService(t *testing.T) {
	service := defaultService()
	prefix := service.Prefix()
	prefixRegistry := NewPrefixes()
	prefixRegistry.ReloadServices([]key.Service{service})

	// Enable the service first, then disable it, and check that there are no
	// active services.
	require.NoError(t, prefixRegistry.UpdateService(service, ServiceEnabled))
	require.NoError(t, prefixRegistry.UpdateService(service, ServiceDisabled))
	require.Contains(t, prefixRegistry.prefixes, prefix)
	require.Contains(t, prefixRegistry.prefixes[prefix].services, service)
	assert.Equal(t, ServiceDisabled, prefixRegistry.prefixes[prefix].services[service])
	assert.Equal(t, 0, prefixRegistry.prefixes[prefix].active)
}

// TestPrefix_EnableNotExistingService tests enabling a service that doesn't
// exist in the prefix registry.
func TestPrefix_EnableNotExistingService(t *testing.T) {
	service := defaultService()
	prefixRegistry := NewPrefixes()
	prefixRegistry.ReloadServices([]key.Service{service})

	// Try to enable a non-existing service and expect an error.
	notExistingService := key.Service{Addr: netip.MustParseAddr("127.0.0.2"), Port: 80, Proto: "TCP"}
	err := prefixRegistry.UpdateService(notExistingService, ServiceEnabled)
	assert.ErrorIs(t, err, ErrPrefixNotFound)
}

// TestPrefix_ReloadServices_UpdateReload_AddServices_WithoutAnnounces tests
// reloading services with additional services without triggering announces.
func TestPrefix_ReloadServices_UpdateReload_AddServices_WithoutAnnounces(t *testing.T) {
	services := defaultServices()
	prefixRegistry := NewPrefixes()
	prefixRegistry.ReloadServices(services)

	// Adding a new service to an existing prefix and a new service to a new
	// prefix.
	services = append(
		services,
		key.Service{Addr: netip.MustParseAddr("127.0.0.1"), Port: 8080, Proto: "TCP"},       // adding service for existing prefix
		key.Service{Addr: netip.MustParseAddr("2001:dead:beef::2"), Port: 80, Proto: "TCP"}, // adding service for new prefix
	)
	prefixRegistry.ReloadServices(services)

	prefixes := map[netip.Prefix]*prefixState{
		netip.MustParsePrefix("127.0.0.1/32"):          newState([]key.Service{services[0], services[1], services[4]}),
		netip.MustParsePrefix("2001:dead:beef::1/128"): newState([]key.Service{services[2], services[3]}),
		netip.MustParsePrefix("2001:dead:beef::2/128"): newState([]key.Service{services[5]}),
	}
	assert.Equal(t, prefixes, prefixRegistry.prefixes)
}

// TestPrefix_ReloadServices_UpdateReload_RemoveServices_WithoutAnnounces tests
// reloading services with some removed without triggering announces.
func TestPrefix_ReloadServices_UpdateReload_RemoveServices_WithoutAnnounces(t *testing.T) {
	services := defaultServices()
	prefixRegistry := NewPrefixes()
	prefixRegistry.ReloadServices(services)

	// Check that quorum equals the number of services for prefix.
	assert.Equal(t, 2, prefixRegistry.prefixes[netip.MustParsePrefix("127.0.0.1/32")].quorum)

	// Remove all but one service and reload the registry, then check the
	// updated prefixes.
	services = []key.Service{services[0]} // Keep only Service1
	prefixRegistry.ReloadServices(services)

	prefixes := map[netip.Prefix]*prefixState{
		netip.MustParsePrefix("127.0.0.1/32"): newState(services),
	}
	assert.Equal(t, prefixes, prefixRegistry.prefixes)
	assert.Equal(t, 1, prefixRegistry.prefixes[netip.MustParsePrefix("127.0.0.1/32")].quorum)
}

// TestPrefix_Announce_Enable tests enabling services and verifying that the
// prefix becomes ready.
func TestPrefix_Announce_Enable(t *testing.T) {
	services := defaultServices()
	prefixRegistry := NewPrefixes()
	prefixRegistry.ReloadServices(services)

	// Enable services and check that the prefix status becomes Ready.
	prefix := services[0].Prefix()
	require.NoError(t, prefixRegistry.UpdateService(services[0], ServiceEnabled))
	require.NoError(t, prefixRegistry.UpdateService(services[1], ServiceEnabled))

	assert.Equal(t, Ready, prefixRegistry.prefixes[prefix].Status())
	assert.Equal(t, 2, prefixRegistry.prefixes[prefix].active)
}

// TestPrefix_Announce_Disable tests disabling services and verifying that the
// prefix becomes unready.
func TestPrefix_Announce_Disable(t *testing.T) {
	services := defaultServices()
	prefixRegistry := NewPrefixes()
	prefixRegistry.ReloadServices(services)

	// Enable services first, then disable one and check that the prefix status
	// becomes Unready.
	prefix := services[0].Prefix()
	require.NoError(t, prefixRegistry.UpdateService(services[0], ServiceEnabled))
	require.NoError(t, prefixRegistry.UpdateService(services[1], ServiceEnabled))
	assert.Equal(t, Ready, prefixRegistry.prefixes[prefix].Status())

	require.NoError(t, prefixRegistry.UpdateService(services[1], ServiceDisabled))
	assert.Equal(t, Unready, prefixRegistry.prefixes[prefix].Status())
}

// TestPrefix_ReloadServices_UpdateReload_AddServices_WithAnnounces tests adding
// new services to the registry with announces.
func TestPrefix_ReloadServices_UpdateReload_AddServices_WithAnnounces(t *testing.T) {
	services := defaultServices()
	prefixRegistry := NewPrefixes()
	prefixRegistry.ReloadServices(services)

	// Enable services and check that the prefix status is Ready.
	prefix := services[0].Prefix()
	require.NoError(t, prefixRegistry.UpdateService(services[0], ServiceEnabled))
	require.NoError(t, prefixRegistry.UpdateService(services[1], ServiceEnabled))
	assert.Equal(t, Ready, prefixRegistry.prefixes[prefix].Status())

	// Add a new service for the existing prefix, reload the registry, and check
	// that the prefix status becomes Unready.
	services = append(
		services,
		key.Service{Addr: netip.MustParseAddr("127.0.0.1"), Port: 8080, Proto: "TCP"}, // adding service for existing prefix
	)
	prefixRegistry.ReloadServices(services)

	assert.Equal(t, Unready, prefixRegistry.prefixes[prefix].Status())
}

// TestPrefix_ReloadServices_UpdateReload_RemoveEnabledService_WithAnnounces
// verifies the removal of an enabled service from the prefix registry with
// announce handling. Initially, two services are registered and enabled. Then,
// one of the services is removed, and it is checked that the prefix status
// remains "Ready" (since one service is still active).
func TestPrefix_ReloadServices_UpdateReload_RemoveEnabledService_WithAnnounces(t *testing.T) {
	services := defaultServices()
	prefixRegistry := NewPrefixes()
	prefixRegistry.ReloadServices(services)

	prefix := services[0].Prefix()
	require.NoError(t, prefixRegistry.UpdateService(services[0], ServiceEnabled))
	require.NoError(t, prefixRegistry.UpdateService(services[1], ServiceEnabled))
	assert.Equal(t, Ready, prefixRegistry.prefixes[prefix].Status())

	services = []key.Service{services[0]} // Keep only Service1
	prefixRegistry.ReloadServices(services)

	assert.Equal(t, Ready, prefixRegistry.prefixes[prefix].Status())
}

// TestPrefix_ReloadServices_UpdateReload_RemoveDisabledService_WithAnnounces
// tests the removal of a disabled service from the prefix registry with
// announce handling. Initially, two services are registered; the first is
// enabled, and the second is disabled. After removing the disabled service, the
// status of the prefix is expected to change to "Ready".
func TestPrefix_ReloadServices_UpdateReload_RemoveDisabledService_WithAnnounces(t *testing.T) {
	services := defaultServices()
	prefixRegistry := NewPrefixes()
	prefixRegistry.ReloadServices(services)

	prefix := services[0].Prefix()
	require.NoError(t, prefixRegistry.UpdateService(services[0], ServiceEnabled))
	require.NoError(t, prefixRegistry.UpdateService(services[1], ServiceDisabled))
	assert.Equal(t, Unready, prefixRegistry.prefixes[prefix].Status())

	services = []key.Service{services[0]} // Keep only Service1
	prefixRegistry.ReloadServices(services)

	assert.Equal(t, Ready, prefixRegistry.prefixes[prefix].Status())
}

// TestPrefix_UpdateServices_Event checks that events are correctly generated
// when services are updated. The test verifies that after enabling two
// services, the expected event is recorded in the prefix registry.
func TestPrefix_UpdateServices_Event(t *testing.T) {
	services := defaultServices()
	prefixRegistry := NewPrefixes()
	prefixRegistry.ReloadServices(services)

	require.NoError(t, prefixRegistry.UpdateService(services[0], ServiceEnabled))
	assert.Empty(t, prefixRegistry.Events())

	require.NoError(t, prefixRegistry.UpdateService(services[1], ServiceEnabled))

	prefix := services[0].Prefix()
	expectedEvents := map[netip.Prefix]PrefixStatus{
		prefix: Ready,
	}
	assert.Equal(t, expectedEvents, prefixRegistry.Events())
}

// TestPrefix_ReloadServices_UpdateReload_RemoveAllEnabledServicesForPrefix_WithAnnounces
// ensures that when all enabled services for a prefix are removed, the prefix
// is also removed from the registry, and appropriate events are generated.
func TestPrefix_ReloadServices_UpdateReload_RemoveAllEnabledServicesForPrefix_WithAnnounces(t *testing.T) {
	services := defaultServices()
	prefixRegistry := NewPrefixes()
	prefixRegistry.ReloadServices(services)

	prefix := services[0].Prefix()
	require.NoError(t, prefixRegistry.UpdateService(services[0], ServiceEnabled))
	require.NoError(t, prefixRegistry.UpdateService(services[1], ServiceEnabled))
	assert.Equal(t, Ready, prefixRegistry.prefixes[prefix].Status())
	expectedEvents := map[netip.Prefix]PrefixStatus{
		prefix: Ready,
	}
	assert.Equal(t, expectedEvents, prefixRegistry.Events())

	services = []key.Service{services[2], services[3]} // Keep only Service3 and Service4
	prefixRegistry.ReloadServices(services)

	assert.NotContains(t, prefixRegistry.prefixes, prefix)

	expectedEvents = map[netip.Prefix]PrefixStatus{
		prefix: Unready,
	}
	assert.Equal(t, expectedEvents, prefixRegistry.Events())
}
