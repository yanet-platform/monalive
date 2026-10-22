package real

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	log "go.uber.org/zap"
)

type transitionHistogram struct {
	values []float64
}

func (h *transitionHistogram) Observe(value float64) {
	h.values = append(h.values, value)
}

func TestInitialTransitionTimestamp(t *testing.T) {
	before := time.Now()
	real := New(&Config{}, nil, log.NewNop())
	require.False(t, real.state.TransitionTimestamp.Before(before))
	require.False(t, real.state.TransitionTimestamp.After(time.Now()))

	histogram := &transitionHistogram{}
	real.SetMetrics(SetRealTransitionPeriodMetric(histogram))
	real.updateTransitionTimestamp()
	require.Len(t, histogram.values, 1)
	require.GreaterOrEqual(t, histogram.values[0], 0.0)
	require.LessOrEqual(t, histogram.values[0], time.Since(before).Seconds())
}
