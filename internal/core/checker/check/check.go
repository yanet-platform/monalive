// Package check contains various implementations of the health check.
package check

import (
	"github.com/yanet-platform/monalive/internal/types/weight"
)

// Metadata holds the status and weight information for a health check.
type Metadata struct {
	Alive  bool
	Weight weight.Weight
	// Force indicates whether the check result shouls be processed in any
	// scenario.
	Force bool
}

// SetInactive updates the Metadata to indicate a failure in the health check. It
// sets Alive to false and Weight to [weight.Omitted].
func (m *Metadata) SetInactive() {
	m.Alive = false
	m.Weight = weight.Omitted
}
