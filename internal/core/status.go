package core

import (
	"github.com/yanet-platform/monalive/internal/core/service"
)

// Status retrieves the current status of all services managed by the Core
// instance. It returns a slice of [service.Status] snapshots
// representing the status of each service.
func (m *Core) Status() []service.Status {
	// Lock the services mutex to ensure thread-safe access.
	m.servicesMu.Lock()
	defer m.servicesMu.Unlock()

	// Create a slice to hold the status of each service.
	status := make([]service.Status, 0, len(m.services))
	// Iterate over each service and append its status to the slice.
	for _, service := range m.services {
		status = append(status, service.Status())
	}

	// Return the collected service status information.
	return status
}
