package announcer

import (
	"fmt"
	"net/netip"
	"sync"

	"github.com/yanet-platform/monalive/internal/types/key"
	event "github.com/yanet-platform/monalive/internal/utils/event_registry"
)

// ErrPrefixNotFound is returned when a requested prefix cannot be found.
var ErrPrefixNotFound = fmt.Errorf("prefix not found")

// PrefixKey is a key for event registry for passing prefix events with it's
// associated group.
type PrefixKey struct {
	Prefix netip.Prefix
	Group  string
}

// The prefix status represents the readiness of the prefix to be announced.
type PrefixStatus bool

const (
	// Ready indicates that the prefix is ready and active.
	Ready PrefixStatus = true

	// Unready indicates that the prefix is not ready or inactive.
	Unready PrefixStatus = false
)

// Merge merges the current status with a new status and determines if the
// current status should be removed.
//
// This method implemented in order to PrefixStatus can be used in
// [event.Registry].
func (status PrefixStatus) Merge(newStatus PrefixStatus) (result PrefixStatus, remove bool) {
	if status != newStatus {
		return Unready, true
	}
	return newStatus, false
}

type ServiceStatus bool

const (
	// ServiceEnabled indicates that the service is ready and active.
	ServiceEnabled ServiceStatus = true

	// ServiceDisabled indicates that the service is not ready or inactive.
	ServiceDisabled ServiceStatus = false
)

// Merge merges the current service status with a new ones and determines if the
// event should be removed from the registry.
//
// This method implemented in order to ServiceStatus can be used in
// [event.Registry].
func (status ServiceStatus) Merge(newStatus ServiceStatus) (result ServiceStatus, remove bool) {
	if status != newStatus {
		// It is correct to return any status as a result here, because the
		// event will be removed anyway. Return [ServiceDisabled], as it's more
		// secure.
		return ServiceDisabled, true
	}
	return newStatus, false
}

// Prefixes manages the state of prefixes and stores events related to them.
type Prefixes struct {
	prefixes map[netip.Prefix]*prefixState            // stores the state of each prefix
	events   *event.Registry[PrefixKey, PrefixStatus] // stores prefix status updates using the prefix as a key
	mu       sync.RWMutex                             // protects access to the prefixes map to ensure safe concurrent access
}

// NewPrefixes creates a new instance of Prefixes.
func NewPrefixes() *Prefixes {
	return &Prefixes{
		prefixes: make(map[netip.Prefix]*prefixState),
		events:   event.NewRegistry[PrefixKey, PrefixStatus](),
	}
}

// ReloadServices updates the services associated with each prefix. It applies
// the new services and removes any prefixes that are no longer in use.
func (m *Prefixes) ReloadServices(services map[key.Service]string) {
	// Lock the mutex for writing since we are modifying the prefixes map.
	m.mu.Lock()
	defer m.mu.Unlock()

	// Temporarily stores the new set of services mapped by their prefixes.
	prefixServices := make(map[netip.Prefix][]key.Service)
	// Construct mapping of prefixes to their new announce group.
	prefixGroup := make(map[netip.Prefix]string)
	for service, group := range services {
		prefix := service.Prefix()
		prefixServices[prefix] = append(prefixServices[prefix], service)
		prefixGroup[prefix] = group
	}

	// Iterate over the existing prefixes to update or remove them.
	for prefix, state := range m.prefixes {
		switch newServices, exists := prefixServices[prefix]; exists {
		case true: // if the prefix is still in use, update its services
			oldGroup, newGroup := state.Group(), prefixGroup[prefix]

			// Construct a key for the prefix for possible prefix events. By
			// now, it should contain the old announce group for the correct
			// event handling.
			prefixKey := PrefixKey{Prefix: prefix, Group: oldGroup}

			if newGroup != oldGroup {
				// If the group changes, record the removal event for the old
				// prefix key.
				m.events.Store(prefixKey, Unready)
				// Update announce group of the prefix.
				state.UpdateGroup(newGroup)
				// Do not forget to update the prefixKey to handle events
				// correctly in the future.
				prefixKey.Group = newGroup
			}

			oldStatus := state.Status()
			state.ApplyServices(newServices)
			newStatus := state.Status()

			if newStatus != oldStatus || newGroup != oldGroup {
				// If the status changes, record the event. Also send the
				// current prefix status if the announce group has changed.
				m.events.Store(prefixKey, newStatus)
			}

			// Remove the prefix from newPrefixes as it has been processed.
			delete(prefixServices, prefix)

		case false: // if the prefix no longer in use, remove it
			if state.Status() == Ready {
				// If the prefix was announced, record announce removal event.
				prefixKey := PrefixKey{Prefix: prefix, Group: state.Group()}
				m.events.Store(prefixKey, Unready)
			}
			// Remove the prefix from the prefixes map.
			delete(m.prefixes, prefix)
		}
	}

	// Add any new prefixes.
	for prefix, services := range prefixServices {
		group := prefixGroup[prefix]
		m.prefixes[prefix] = newState(group, services)
	}
}

// UpdateService updates the status of a specific service associated with a
// prefix. It returns an error if the prefix is not found.
func (m *Prefixes) UpdateService(service key.Service, status ServiceStatus) error {
	// Lock the mutex for reading since we are only reading from the prefixes
	// map.
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Retrieve the prefix associated with the service.
	prefix := service.Prefix()

	// Find the state associated with the prefix.
	state := m.prefixes[prefix]
	if state == nil {
		// Return an error if the prefix does not exist.
		return fmt.Errorf("%w, service: %s", ErrPrefixNotFound, service)
	}

	// Update the service status and record any status change as an event.
	oldStatus := state.Status()
	if newStatus := state.UpdateService(service, status); newStatus != oldStatus {
		prefixKey := PrefixKey{Prefix: prefix, Group: state.Group()}
		m.events.Store(prefixKey, newStatus)
	}

	return nil
}

// StatusFor returns the current status of passed prefixes.
func (m *Prefixes) StatusFor(prefixes []netip.Prefix) map[netip.Prefix]PrefixStatus {
	// Lock the mutex for reading since we are only reading from the prefixes
	// map.
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a map to store the status of passed prefixes and fill it.
	status := make(map[netip.Prefix]PrefixStatus, len(prefixes))
	for _, prefix := range prefixes {
		switch state, exists := m.prefixes[prefix]; exists {
		case true:
			status[prefix] = state.Status()
		case false:
			// If the prefix does not exist, use [Unready] status, as it's more
			// secure.
			status[prefix] = Unready
		}
	}

	return status
}

// Events retrieves and clears the events storage.
func (m *Prefixes) Events() map[PrefixKey]PrefixStatus {
	return m.events.Flush()
}

// prefixState manages the state of services associated with a prefix. It tracks
// the active services and determines if the prefix is ready based on the
// quorum.
type prefixState struct {
	group    string                        // announce group of the prefix
	services map[key.Service]ServiceStatus // keeps set of the services for this prefix
	active   int                           // the number of active services for this prefix
	quorum   int                           // the number of services required for the prefix to be considered ready
	mu       sync.RWMutex                  // protects access to the prefixState to ensure safe concurrent access
}

// newState creates a new prefixState with the specified quorum.
func newState(group string, services []key.Service) *prefixState {
	servicesSet := make(map[key.Service]ServiceStatus)
	for _, service := range services {
		servicesSet[service] = ServiceDisabled
	}
	return &prefixState{
		group:    group,
		services: servicesSet,
		active:   0,
		// Number of services used as a quorum because prefix announce should
		// not be raised until all dependent services are ready.
		quorum: len(servicesSet),
	}
}

// ApplyServices updates the active services for the prefix based on the
// provided list of new services.
func (m *prefixState) ApplyServices(newServices []key.Service) {
	// Lock the mutex for writing since we are modifying the activeServices map.
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a new set of services based on the provided list.
	newServicesSet := make(map[key.Service]ServiceStatus, len(newServices))
	// Count the number of active services in the new set of services.
	active := 0
	for _, service := range newServices {
		// All new services are disabled by default.
		status := ServiceDisabled

		// If the service was presented earlier and has [ServiceEnabled] status,
		// update its status.
		if knownStatus, exists := m.services[service]; exists && knownStatus == ServiceEnabled {
			status = ServiceEnabled
			active++
		}
		newServicesSet[service] = status
	}

	// Update the active services map with the new set of services.
	m.services = newServicesSet
	// Update the active count based on the new set of services.
	m.active = active
	// Update the quorum based on the number of new services.
	m.quorum = len(newServices)
}

// UpdateService updates the status of a specific service within the prefix. It
// returns the new status of the prefix.
func (m *prefixState) UpdateService(service key.Service, status ServiceStatus) (newStatus PrefixStatus) {
	// Lock the mutex for writing since we are modifying the activeServices map.
	m.mu.Lock()
	defer m.mu.Unlock()

	if currStatus, exists := m.services[service]; !exists || currStatus == status {
		// Return the current status of the prefix if the service does not exist
		// or current status is the same as the new status.
		return m.status()
	}

	// Update the active count based on the new status of the service.
	switch status {
	case ServiceEnabled:
		m.active++
	case ServiceDisabled:
		m.active--
	}

	// Update the status of the service.
	m.services[service] = status

	// Return the updated status of the prefix.
	return m.status()
}

// Status returns the current status of the prefix.
func (m *prefixState) Status() PrefixStatus {
	// Lock the mutex for reading since we are only reading from the
	// activeServices map.
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Calculate and return the status of the prefix.
	return m.status()
}

// Group returns the announce group of the prefix.
func (m *prefixState) Group() string {
	// Lock the mutex for reading since we are only reading group value.
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.group
}

// UpdateGroup updates the announce group of the prefix.
func (m *prefixState) UpdateGroup(group string) {
	// Lock the mutex for writing since we are modifying the group value.
	m.mu.Lock()
	defer m.mu.Unlock()
	m.group = group
}

// status is a helper method to determine if the prefix is ready based on the
// number of active services and the quorum. It assumes the prefixState mutex is
// already held.
func (m *prefixState) status() PrefixStatus {
	// A prefix is considered ready if the number of active services meets the
	// quorum.
	if m.quorum == m.active && m.active != 0 {
		return Ready
	}
	return Unready
}
