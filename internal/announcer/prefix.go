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

// PrefixesGroups represents a collection of Prefixes, grouped by an announce
// group names.
type PrefixesGroups map[string]*Prefixes

// NewPrefixesGroups creates a new PrefixesGroups instance initialized with the
// provided group names.
func NewPrefixesGroups(groups []string) PrefixesGroups {
	prefixesGroups := make(PrefixesGroups, len(groups))
	for _, group := range groups {
		prefixesGroups[group] = NewPrefixes()
	}
	return prefixesGroups
}

// GetGroup retrieves the Prefixes associated with the given group name.
//
// This function returns nil if provided group is not found.
func (m PrefixesGroups) GetGroup(group string) *Prefixes {
	return m[group]
}

// AnnounceGroupByPrefix is a concurent safe кey-value store for mapping
// prefixes to their corresponding announce group.
type AnnounceGroupByPrefix struct {
	store map[netip.Prefix]string
	mu    sync.RWMutex // to protect concurent access to the store map
}

// NewAnnounceGroupByPrefix creates a new instance of AnnounceGroupByPrefix.
func NewAnnounceGroupByPrefix() *AnnounceGroupByPrefix {
	return &AnnounceGroupByPrefix{
		store: make(map[netip.Prefix]string),
	}
}

// GetAnnounceGroup retrieves the announce group associated with the given
// prefix.
func (m *AnnounceGroupByPrefix) GetAnnounceGroup(prefix netip.Prefix) (group string, exists bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	group, exists = m.store[prefix]
	return group, exists
}

// Update updates the mapping of prefixes to their corresponding announce group.
func (m *AnnounceGroupByPrefix) Update(groupByPrefix map[netip.Prefix]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store = groupByPrefix
}

// Prefixes manages the state of prefixes within a single announce group and
// stores events related to them.
type Prefixes struct {
	prefixes map[netip.Prefix]*prefixState               // stores the state of each prefix
	events   *event.Registry[netip.Prefix, PrefixStatus] // stores prefix status updates using the prefix as a key
	mu       sync.RWMutex                                // protects access to the prefixes map to ensure safe concurrent access
}

// NewPrefixes creates a new instance of Prefixes.
func NewPrefixes() *Prefixes {
	return &Prefixes{
		prefixes: make(map[netip.Prefix]*prefixState),
		events:   event.NewRegistry[netip.Prefix, PrefixStatus](),
	}
}

// ReloadServices updates the services associated with each prefix. It applies
// the new services and removes any prefixes that are no longer in use.
func (m *Prefixes) ReloadServices(services []key.Service) {
	// Lock the mutex for writing since we are modifying the prefixes map.
	m.mu.Lock()
	defer m.mu.Unlock()

	// newPrefixes temporarily stores the new set of services mapped by their
	// prefixes.
	newPrefixes := make(map[netip.Prefix][]key.Service)
	for _, service := range services {
		prefix := service.Prefix()
		newPrefixes[prefix] = append(newPrefixes[prefix], service)
	}

	// Iterate over the existing prefixes to update or remove them.
	for prefix, state := range m.prefixes {
		if newServices, exists := newPrefixes[prefix]; exists {
			// If the prefix is still in use, update its services.
			oldStatus := state.Status()
			state.ApplyServices(newServices)
			if newStatus := state.Status(); newStatus != oldStatus {
				// If the status changes, record the event.
				m.events.Store(prefix, newStatus)
			}

			// Remove the prefix from newPrefixes as it has been processed.
			delete(newPrefixes, prefix)

		} else {
			if state.Status() == Ready {
				// If the prefix was announced, record announce removal event.
				m.events.Store(prefix, Unready)
			}
			// Remove the prefix from the prefixes map.
			delete(m.prefixes, prefix)
		}
	}

	// Add any new prefixes.
	for prefix, newServices := range newPrefixes {
		m.prefixes[prefix] = newState(newServices)
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
		m.events.Store(prefix, newStatus)
	}

	return nil
}

// Status returns the current status of all prefixes.
func (m *Prefixes) Status() map[netip.Prefix]PrefixStatus {
	// Lock the mutex for reading since we are only reading from the prefixes
	// map.
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a map to store the status of each prefix and fill it.
	status := make(map[netip.Prefix]PrefixStatus, len(m.prefixes))
	for prefix, state := range m.prefixes {
		status[prefix] = state.Status()
	}

	return status
}

// Events retrieves and clears the events storage.
func (m *Prefixes) Events() map[netip.Prefix]PrefixStatus {
	return m.events.Flush()
}

// prefixState manages the state of services associated with a prefix. It tracks
// the active services and determines if the prefix is ready based on the
// quorum.
type prefixState struct {
	services map[key.Service]ServiceStatus // keeps set of the services for this prefix
	active   int                           // the number of active services for this prefix
	quorum   int                           // the number of services required for the prefix to be considered ready
	mu       sync.RWMutex                  // protects access to the prefixState to ensure safe concurrent access
}

// newState creates a new prefixState with the specified quorum.
func newState(services []key.Service) *prefixState {
	servicesSet := make(map[key.Service]ServiceStatus)
	for _, service := range services {
		servicesSet[service] = ServiceDisabled
	}
	return &prefixState{
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
	for _, service := range newServices {
		newServicesSet[service] = ServiceDisabled
	}

	// Remove any services that are no longer active.
	for service, status := range m.services {
		if _, exists := newServicesSet[service]; !exists {
			if status == ServiceEnabled {
				m.active--
			}
		}
	}

	// Update the active services map with the new set of services.
	m.services = newServicesSet
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
