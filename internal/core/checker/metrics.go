package checker

import (
	"sync"

	"github.com/yanet-platform/monalive/internal/monitoring/metrics"
)

type SetMetricFunc func(m *Metrics)

type Metrics struct {
	responseTime metrics.Histogram
	errors       metrics.CounterVec

	isBlocked bool
	mu        sync.Mutex
}

func NewMetrics() *Metrics {
	return &Metrics{
		responseTime: &metrics.NopHistogram{},
		errors:       &metrics.NopCounterVec{},
	}
}

func (m *Metrics) ResponseTime() metrics.Histogram {
	return m.responseTime
}

func (m *Metrics) Errors() metrics.CounterVec {
	return m.errors
}

func (m *Metrics) Block() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isBlocked = true
}

func SetResponceTimeMetric(hist metrics.Histogram) SetMetricFunc {
	return func(m *Metrics) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if !m.isBlocked {
			m.responseTime = hist
		}

	}
}

func SetErrorsMetric(counterVec metrics.CounterVec) SetMetricFunc {
	return func(m *Metrics) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if !m.isBlocked {
			m.errors = counterVec
		}
	}
}
