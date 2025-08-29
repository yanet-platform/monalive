package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yanet-platform/monalive/internal/core/checker/check"
	"github.com/yanet-platform/monalive/internal/monitoring/xlog"
	"github.com/yanet-platform/monalive/internal/scheduler"
	"github.com/yanet-platform/monalive/internal/types/weight"
	"github.com/yanet-platform/monalive/internal/types/xevent"
	"github.com/yanet-platform/monalive/internal/utils/xnet"
)

// testHandler is a mock handler used for testing. It stores the last event
// received in its Handle method for later retrieval.
type testHandler struct {
	event *xevent.Event
}

// Handle stores the given event in the testHandler. This method simulates the
// behavior of a real event handler in tests by capturing the event for later
// examination.
func (m *testHandler) Handle(event *xevent.Event) {
	m.event = event
}

// Event returns the last stored event and clears it from the testHandler. This
// method allows tests to retrieve and assert on the captured event.
func (m *testHandler) Event() *xevent.Event {
	defer func() { m.event = nil }()
	return m.event
}

// defaultChecker creates and returns a new Checker instance with default
// settings. The checker is initialized with dynamic weight enabled and a
// default weight coefficient. It also sets up a default scheduler configuration
// with retries set to 0.
func defaultChecker(handler xevent.Handler, weight weight.Weight) *Checker {
	schedConfig := scheduler.Config{}
	schedConfig.Default()
	*schedConfig.Retries = 0

	config := &Config{
		Type:      HTTPChecker,
		Scheduler: schedConfig,
		CheckConfig: check.Config{
			WeightControl: check.WeightControl{
				DynamicWeight:      true,
				DynamicWeightCoeff: 30, // 30%
			},
		},
	}

	return New(config, handler, weight, xnet.ForwardingData{}, xlog.NewNopLogger())
}

// WithRetries configures the Checker to use retries by setting the retries
// count to 1. It returns the modified Checker instance.
func (m *Checker) WithRetries() *Checker {
	retries := 1
	m.config.Retries = &retries
	return m
}

// WithoutDynWeight disables dynamic weight adjustment by setting the
// DynamicWeight configuration to false. It returns the modified Checker
// instance.
func (m *Checker) WithoutDynWeight() *Checker {
	m.config.DynamicWeight = false
	return m
}

// TestProcessCheck_EnableDisabled tests the scenario where a Checker is
// initially disabled and then receives a successful health check. It verifies
// that the checker is enabled, and the weight and state are updated correctly.
// It also checks that an enable event is generated and contains the correct
// information.
func TestProcessCheck_EnableDisabled(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	checker := defaultChecker(handler.Handle, initWeight)

	md := check.Metadata{Alive: true, Weight: 1}
	checker.ProcessCheck(md, nil)

	// Verify that the checker's state weight is unchanged and it is marked as
	// alive.
	state := checker.State()
	assert.Equal(t, initWeight, state.Weight)
	assert.Equal(t, true, state.Alive)
	// Ensure that an enable event was generated and contains the expected
	// details.
	event := handler.Event()
	require.NotNil(t, event)
	assert.Equal(t, xevent.Enable, event.Type)
	assert.Equal(t, initWeight, event.New.Weight)
}

// TestProcessCheck_EnableEnabled tests the scenario where a Checker is enabled,
// and then receives another successful health check while it is already
// enabled. It verifies that the checker's state remains unchanged and no
// redundant events are generated.
func TestProcessCheck_EnableEnabled(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	checker := defaultChecker(handler.Handle, initWeight)

	md := check.Metadata{Alive: true, Weight: 1}
	checker.ProcessCheck(md, nil)

	// Flush already processed event to ensure we are testing the correct state.
	_ = handler.Event()

	checker.ProcessCheck(md, nil)

	// Verify that the checker's state weight is unchanged and it remains alive.
	state := checker.State()
	assert.Equal(t, initWeight, state.Weight)
	assert.Equal(t, true, state.Alive)
	// Ensure no new event is generated as the state did not change.
	require.Nil(t, handler.Event())
}

// TestProcessCheck_DisableDisabled tests the scenario where a Checker is
// initially enabled and then receives a failed health check. It verifies that
// the checker is disabled, the weight is set to omitted, and a disable event is
// generated with the correct information.
func TestProcessCheck_DisableDisabled(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	checker := defaultChecker(handler.Handle, initWeight)

	md := check.Metadata{Alive: false, Weight: weight.Omitted}
	checker.ProcessCheck(md, fmt.Errorf("failed check"))

	// Verify that the checker is marked as disabled and weight is omitted.
	state := checker.State()
	assert.Equal(t, initWeight, state.Weight)
	assert.Equal(t, false, state.Alive)
	// Ensure that a disable event was generated and contains the expected
	// details.
	event := handler.Event()
	require.NotNil(t, event)
	assert.Equal(t, xevent.Disable, event.Type)
	assert.Equal(t, weight.Omitted, event.New.Weight)
}

// TestProcessCheck_DisableEnabled tests the scenario where a Checker is
// initially enabled, then receives a failed health check, and is subsequently
// disabled. It verifies that the checker is disabled, and a disable event is
// generated with the correct information.
func TestProcessCheck_DisableEnabled(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	checker := defaultChecker(handler.Handle, initWeight)

	md := check.Metadata{Alive: true, Weight: 1}
	checker.ProcessCheck(md, nil)

	// Flush already processed event to ensure we are testing the correct state.
	_ = handler.Event()

	md.SetInactive()
	checker.ProcessCheck(md, fmt.Errorf("failed check"))

	// Verify that the checker is marked as disabled and weight is unchanged.
	state := checker.State()
	assert.Equal(t, initWeight, state.Weight)
	assert.Equal(t, false, state.Alive)
	// Ensure that a disable event was generated and contains the expected
	// details.
	event := handler.Event()
	require.NotNil(t, event)
	assert.Equal(t, xevent.Disable, event.Type)
	assert.Equal(t, weight.Omitted, event.New.Weight)
}

// TestProcessCheck_DisableEnabled_WithRetries tests the scenario where a
// Checker is initially enabled, then receives a failed health check, and is
// configured to use retries. It verifies that the checker remains enabled until
// the retry limit is exceeded, at which point it is disabled and a disable
// event is generated.
func TestProcessCheck_DisableEnabled_WithRetries(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	checker := defaultChecker(handler.Handle, initWeight).WithRetries()

	md := check.Metadata{Alive: true, Weight: 1}
	checker.ProcessCheck(md, nil)

	// Flush already processed event to ensure we are testing the correct state.
	_ = handler.Event()

	md.SetInactive()
	checker.ProcessCheck(md, fmt.Errorf("failed check"))

	// Verify that the checker remains enabled after the first failed check due
	// to retries.
	state := checker.State()
	assert.Equal(t, initWeight, state.Weight)
	assert.Equal(t, true, state.Alive)
	// Ensure no event is generated yet, as the retry limit has not been
	// reached.
	require.Nil(t, handler.Event())

	checker.ProcessCheck(md, fmt.Errorf("failed check"))

	// Verify that the checker is marked as disabled and weight is unchanged
	// after retries.
	state = checker.State()
	assert.Equal(t, initWeight, state.Weight)
	assert.Equal(t, false, state.Alive)
	// Ensure that a disable event was generated and contains the expected
	// details.
	event := handler.Event()
	require.NotNil(t, event)
	assert.Equal(t, xevent.Disable, event.Type)
	assert.Equal(t, weight.Omitted, event.New.Weight)
}

// TestProcessCheck_ChangeWeight_DynWeightDisabled tests the scenario where a
// Checker has dynamic weight control disabled. It verifies that the weight does
// not change when the metadata weight changes, as dynamic weight adjustment is
// turned off.
func TestProcessCheck_ChangeWeight_DynWeightDisabled(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	checker := defaultChecker(handler.Handle, initWeight).WithoutDynWeight()

	md := check.Metadata{Alive: true, Weight: 1}
	checker.ProcessCheck(md, nil)

	// Flush already processed event to ensure we are testing the correct state.
	_ = handler.Event()

	md = check.Metadata{Alive: true, Weight: 10}
	checker.ProcessCheck(md, nil)

	// Verify that the weight remains unchanged as dynamic weight adjustment is
	// disabled.
	state := checker.State()
	assert.Equal(t, initWeight, state.Weight)
	assert.Equal(t, true, state.Alive)
	// Ensure no new event is generated as the state did not change.
	require.Nil(t, handler.Event())
}

// TestProcessCheck_IncreaseWeight tests the scenario where a Checker receives
// multiple successful health checks with an initial weight and dynamic weight
// control enabled. It verifies that the weight is increased according to the
// dynamic weight coefficient (30% of the previous weight) with a minimum
// increase of 1. It also checks the state of the checker after each update.
func TestProcessCheck_IncreaseWeight(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	checker := defaultChecker(handler.Handle, initWeight)

	md := check.Metadata{Alive: true, Weight: 10}

	tests := []struct {
		expectedWeight weight.Weight
	}{
		{weight.Weight(2)},  // weight is updated to 2 (1 + max(1, int(1 * 0.3)))
		{weight.Weight(3)},  // weight is updated to 3 (2 + max(1, int(2 * 0.3)))
		{weight.Weight(4)},  // weight is updated to 4 (3 + max(1, int(3 * 0.3)))
		{weight.Weight(5)},  // weight is updated to 5 (4 + max(1, int(4 * 0.3)))
		{weight.Weight(6)},  // weight is updated to 6 (5 + max(1, int(5 * 0.3)))
		{weight.Weight(7)},  // weight is updated to 7 (6 + max(1, int(6 * 0.3)))
		{weight.Weight(9)},  // weight is updated to 9 (7 + max(1, int(7 * 0.3)))
		{weight.Weight(10)}, // weight is capped at 10 to avoid excessive increases
	}

	for _, tt := range tests {
		checker.ProcessCheck(md, nil)
		state := checker.State()
		assert.Equal(t, tt.expectedWeight, state.Weight)
		assert.Equal(t, true, state.Alive)
	}
}

// TestProcessCheck_ReduceWeight tests the scenario where a Checker with an
// initial weight receives multiple successful health checks, with the weight
// being reduced according to the dynamic weight control (30% of the previous
// weight, with a minimum reduction of 1). It verifies the weight decreases
// correctly with each check and that the checker's state remains alive.
func TestProcessCheck_ReduceWeight(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(10)
	checker := defaultChecker(handler.Handle, initWeight)

	md := check.Metadata{Alive: true, Weight: 1}

	tests := []struct {
		expectedWeight weight.Weight
	}{
		{weight.Weight(7)}, // weight is reduced to 7 (10 - max(1, int(10 * 0.3)))
		{weight.Weight(5)}, // weight is reduced to 5 (7 - max(1, int(7 * 0.3)))
		{weight.Weight(4)}, // weight is reduced to 4 (5 - max(1, int(5 * 0.3)))
		{weight.Weight(3)}, // weight is reduced to 3 (4 - max(1, int(4 * 0.3)))
		{weight.Weight(2)}, // weight is reduced to 2 (3 - max(1, int(3 * 0.3)))
		{weight.Weight(1)}, // weight is reduced to 1 (2 - max(1, int(2 * 0.3)))
	}

	for _, tt := range tests {
		checker.ProcessCheck(md, nil)
		state := checker.State()
		assert.Equal(t, tt.expectedWeight, state.Weight)
		assert.Equal(t, true, state.Alive)
	}
}

// TestProcessCheck_ShutdownEnabled tests the scenario where a Checker is
// initially enabled, and a shutdown signal is processed. It verifies that the
// checker state remains unchanged and a shutdown event is generated with the
// correct details.
func TestProcessCheck_ShutdownEnabled(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	checker := defaultChecker(handler.Handle, initWeight)

	md := check.Metadata{Alive: true, Weight: 1}
	checker.ProcessCheck(md, nil)

	// Flush already processed event to ensure we are testing the correct state.
	_ = handler.Event()

	md.SetInactive()
	checker.ProcessCheck(md, ErrShutdown)

	// Shutdown does not change the checker's state, as it will be deleted
	// afterward.
	state := checker.State()
	assert.Equal(t, initWeight, state.Weight)
	assert.Equal(t, true, state.Alive)

	event := handler.Event()
	// Ensure that a shutdown event was generated with the correct details.
	require.NotNil(t, event)
	assert.Equal(t, xevent.Shutdown, event.Type)
	assert.Equal(t, weight.Omitted, event.New.Weight)
}

// TestProcessCheck_ShutdownEnabled_WithRetries tests the scenario where a
// Checker with retry configuration receives a shutdown signal. It verifies that
// the checker state remains unchanged and a shutdown event is generated,
// regardless of the retry count.
func TestProcessCheck_ShutdownEnabled_WithRetries(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	checker := defaultChecker(handler.Handle, initWeight).WithRetries()

	md := check.Metadata{Alive: true, Weight: 1}
	checker.ProcessCheck(md, nil)

	// Flush already processed event to ensure we are testing the correct state.
	_ = handler.Event()

	md.SetInactive()
	checker.ProcessCheck(md, ErrShutdown)

	// Shutdown should occur regardless of retry count, and the checker state
	// remains unchanged.
	state := checker.State()
	assert.Equal(t, initWeight, state.Weight)
	assert.Equal(t, true, state.Alive)

	event := handler.Event()
	// Ensure that a shutdown event was generated with the correct details.
	require.NotNil(t, event)
	assert.Equal(t, xevent.Shutdown, event.Type)
	assert.Equal(t, weight.Omitted, event.New.Weight)
}

// TestProcessCheck_ShutdownDisabled tests the scenario where a Checker is
// initially disabled and receives a shutdown signal. It verifies that the
// checker state remains unchanged and a shutdown event is generated with the
// correct details.
func TestProcessCheck_ShutdownDisabled(t *testing.T) {
	handler := &testHandler{}
	initWeight := weight.Weight(1)
	checker := defaultChecker(handler.Handle, initWeight)

	var md check.Metadata
	md.SetInactive()
	checker.ProcessCheck(md, ErrShutdown)

	// Shutdown does not change the checker's state, as it will be deleted
	// afterward.
	state := checker.State()
	assert.Equal(t, initWeight, state.Weight)
	assert.Equal(t, false, state.Alive)

	event := handler.Event()
	// Ensure that a shutdown event was generated with the correct details.
	require.NotNil(t, event)
	assert.Equal(t, xevent.Shutdown, event.Type)
	assert.Equal(t, weight.Omitted, event.New.Weight)
}
