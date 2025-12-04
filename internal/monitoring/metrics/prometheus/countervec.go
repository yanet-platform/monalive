package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	log "go.uber.org/zap"

	"github.com/yanet-platform/monalive/internal/monitoring/metrics"
)

// counterVec is an alias for [prometheus.CounterVec], existing to make an
// embedded field private
type counterVec = prometheus.CounterVec

// CounterVec is a wrapper for [prometheus.CounterVec] with a logger for
// error handling
type CounterVec struct {
	*counterVec
	log *log.Logger
}

func newCounterVec(opts prometheus.CounterOpts, labelNames []string, logger *log.Logger) *CounterVec {
	return &CounterVec{
		counterVec: prometheus.NewCounterVec(opts, labelNames),
		log:        logger.With(log.String("type", "counter_vec"), log.String("name", opts.Name)),
	}
}

func (m *CounterVec) GetMetricWith(labels metrics.Labels) metrics.Counter {
	counter, err := m.counterVec.GetMetricWith(prometheus.Labels(labels))
	if err != nil {
		m.log.Error("failed to create metric", log.Error(err))
		return &metrics.NopCounter{}
	}

	return counter
}

func (m *CounterVec) CurryWith(labels metrics.Labels) metrics.CounterVec {
	counterVec, err := m.counterVec.CurryWith(prometheus.Labels(labels))
	if err != nil {
		m.log.Error("failed to carry metric", log.Error(err))
		return &metrics.NopCounterVec{}
	}

	return &CounterVec{
		counterVec: counterVec,
		log:        m.log,
	}
}

func (m *CounterVec) DeletePartialMatch(labels metrics.Labels) {
	m.counterVec.DeletePartialMatch(prometheus.Labels(labels))
}

func (m *CounterVec) Delete(labels metrics.Labels) {
	m.counterVec.Delete(prometheus.Labels(labels))
}
