package real

import (
	"sync"

	"github.com/yanet-platform/monalive/internal/monitoring/metrics"
)

type SetMetricFunc func(m *Metrics)

type Metrics struct {
	realTransitionPeriod metrics.Histogram
	realResponseTime     metrics.Histogram
	realErrors           metrics.CounterVec

	isBlocked bool
	mu        sync.Mutex
}

func NewMetrics() *Metrics {
	return &Metrics{
		realTransitionPeriod: &metrics.NopHistogram{},
		realResponseTime:     &metrics.NopHistogram{},
		realErrors:           &metrics.NopCounterVec{},
	}
}

func (m *Metrics) RealErrors() metrics.CounterVec {
	return m.realErrors
}

func (m *Metrics) RealResponseTime() metrics.Histogram {
	return m.realResponseTime
}

func (m *Metrics) RealTransitionPeriod() metrics.Histogram {
	return m.realTransitionPeriod
}

func (m *Metrics) Block() {
	m.mu.Lock()
	m.isBlocked = true
	m.mu.Unlock()
}

func SetRealErrorsMetric(counterVec metrics.CounterVec) SetMetricFunc {
	return func(m *Metrics) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if !m.isBlocked {
			m.realErrors = counterVec
		}

	}
}

func SetRealResponseTimeMetric(hist metrics.Histogram) SetMetricFunc {
	return func(m *Metrics) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if !m.isBlocked {
			m.realResponseTime = hist
		}

	}
}

func SetRealTransitionPeriodMetric(hist metrics.Histogram) SetMetricFunc {
	return func(m *Metrics) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if !m.isBlocked {
			m.realTransitionPeriod = hist
		}
	}
}
