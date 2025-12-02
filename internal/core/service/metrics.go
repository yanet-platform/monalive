package service

import (
	"sync"

	"github.com/yanet-platform/monalive/internal/monitoring/metrics"
)

type SetMetricFunc func(*Metrics)

type Metrics struct {
	reals        metrics.Gauge
	realsEnabled metrics.Gauge

	realsTransitionPeriod metrics.Histogram
	realsResponseTime     metrics.Histogram

	realsErrors metrics.CounterVec

	isBlocked bool
	mu        sync.Mutex
}

func NewMetrics() *Metrics {
	m := &Metrics{
		reals:        &metrics.NopGauge{},
		realsEnabled: &metrics.NopGauge{},

		realsTransitionPeriod: &metrics.NopHistogram{},
		realsResponseTime:     &metrics.NopHistogram{},

		realsErrors: &metrics.NopCounterVec{},
	}

	return m
}

func (m *Metrics) RealsTotal() metrics.Gauge {
	return m.reals
}

func (m *Metrics) RealsEnabled() metrics.Gauge {
	return m.realsEnabled
}

func (m *Metrics) RealsTransitionPeriod() metrics.Histogram {
	return m.realsTransitionPeriod
}

func (m *Metrics) RealsResponseTime() metrics.Histogram {
	return m.realsResponseTime
}

func (m *Metrics) RealsErrors() metrics.CounterVec {
	return m.realsErrors
}

func (m *Metrics) Block() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isBlocked = true
}

func SetRealsEnabledMetric(gauge metrics.Gauge) SetMetricFunc {
	return func(m *Metrics) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if !m.isBlocked {
			m.realsEnabled = gauge
		}

	}
}

func SetRealsMetric(gauge metrics.Gauge) SetMetricFunc {
	return func(m *Metrics) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if !m.isBlocked {
			m.reals = gauge
		}

	}
}

func SetRealsTransitionPeriodMetric(histogram metrics.Histogram) SetMetricFunc {
	return func(m *Metrics) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if !m.isBlocked {
			m.realsTransitionPeriod = histogram
		}
	}
}

func SetRealsResponseTimeMetric(histogram metrics.Histogram) SetMetricFunc {
	return func(m *Metrics) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if !m.isBlocked {
			m.realsResponseTime = histogram
		}
	}
}

func SetRealsErrorsMetric(counterVec metrics.CounterVec) SetMetricFunc {
	return func(m *Metrics) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if !m.isBlocked {
			m.realsErrors = counterVec
		}
	}
}
