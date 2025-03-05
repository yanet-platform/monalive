package eventregistry

import (
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEvent is a simple struct used for testing the Registry.
// It contains a Value and an InitValue, and implements the Comparable
// interface.
type TestEvent struct {
	Value     int
	InitValue int
}

// Merge defines the logic to merge two TestEvent instances.
// If the new event's Value is equal to the InitValue of the existing event,
// the event is removed from the registry.
func (event TestEvent) Merge(newEvent TestEvent) (result TestEvent, remove bool) {
	if newEvent.Value == event.InitValue {
		return TestEvent{}, true
	}

	event.Value = newEvent.Value
	return event, false
}

// TestRegistry_Events verifies the behavior of the Events method in the
// Registry. It checks that events are stored correctly and that merging works
// as expected.
func TestRegistry_Events(t *testing.T) {
	events := NewRegistry[string, TestEvent]()

	// Store several events under the same key, merging them.
	events.Store("test", TestEvent{Value: 1, InitValue: 0})
	events.Store("test", TestEvent{Value: 2, InitValue: 1})
	events.Store("test", TestEvent{Value: 3, InitValue: 2})

	// Retrieve events and verify the stored values.
	data := events.Events()
	require.Contains(t, data, "test")
	assert.Equal(t, 3, data["test"].Value)
	assert.Equal(t, 0, data["test"].InitValue)

	// Store an event that triggers removal due to the Merge logic.
	events.Store("test", TestEvent{Value: 0, InitValue: 3})
	data = events.Events()
	assert.NotContains(t, data, "test")
}

// TestRegistry_Flush verifies the behavior of the Flush method.
// It checks that all events are returned and that the registry is cleared
// afterward.
func TestRegistry_Flush(t *testing.T) {
	events := NewRegistry[string, TestEvent]()

	// Store several events under the same key.
	events.Store("test", TestEvent{Value: 1, InitValue: 0})
	events.Store("test", TestEvent{Value: 2, InitValue: 1})
	events.Store("test", TestEvent{Value: 3, InitValue: 2})

	// Flush the registry and verify that all events are returned.
	data := events.Flush()
	require.Contains(t, data, "test")
	assert.Equal(t, 3, data["test"].Value)
	assert.Equal(t, 0, data["test"].InitValue)

	// Store another event and verify that it is stored in the now-empty
	// registry.
	events.Store("test", TestEvent{Value: 0, InitValue: 3})
	data = events.Events()
	require.Contains(t, data, "test")
	assert.Equal(t, 0, data["test"].Value)
	assert.Equal(t, 3, data["test"].InitValue)
}

// TestRegistry_Process verifies the behavior of the Process method.
// It checks that the processor function is correctly applied to events,
// and that events are removed from the registry when the processor succeeds.
func TestRegistry_Process(t *testing.T) {
	whiteList := []string{"goodKey"} // only keys in the whitelist should be processed
	result := []TestEvent{}          // this slice will store successfully processed events

	// Processor function that only succeeds if the key is in the whitelist.
	processor := func(key string, event TestEvent) error {
		if slices.Contains(whiteList, key) {
			result = append(result, event)
			return nil
		}
		return fmt.Errorf("key %q is not found in white list", key)
	}

	// Create a registry and store events under both a whitelisted and a
	// non-whitelisted key.
	events := NewRegistry[string, TestEvent]()
	events.Store("goodKey", TestEvent{Value: 1, InitValue: 0})
	events.Store("badKey", TestEvent{Value: 1, InitValue: 0})

	// Process the registry with the processor function.
	processed := events.Process(processor)

	// Verify that only the event with the whitelisted key was processed.
	assert.Equal(t, 1, processed)
	assert.Equal(t, result, []TestEvent{{Value: 1, InitValue: 0}})

	// Verify that the event with the non-whitelisted key remains in the registry.
	data := events.Events()
	require.Contains(t, data, "badKey")
	assert.Equal(t, 1, data["badKey"].Value)
	assert.Equal(t, 0, data["badKey"].InitValue)
}
