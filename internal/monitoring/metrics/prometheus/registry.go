package prometheus

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type Registry[T prometheus.Collector] struct {
	registry *prometheus.Registry
	metrics  map[string]T
	mu       sync.Mutex
}

type MetricConstructorFunc[T prometheus.Collector] func() T

func newRegistry[T prometheus.Collector](registry *prometheus.Registry) *Registry[T] {
	return &Registry[T]{
		registry: registry,
		metrics:  make(map[string]T),
	}
}

func (m *Registry[T]) GetOrCreateMetric(name string, constructor MetricConstructorFunc[T]) (T, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if metric, exists := m.metrics[name]; exists {
		return metric, nil
	}

	metric := constructor()
	if err := m.registry.Register(metric); err != nil {
		return metric, err
	}

	m.metrics[name] = metric

	return metric, nil
}

func (m *Registry[T]) DeleteMetric(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if metric, exists := m.metrics[name]; exists {
		m.registry.Unregister(metric)
		delete(m.metrics, name)
	}
}

func (m *Registry[T]) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, metric := range m.metrics {
		m.registry.Unregister(metric)
	}
	m.metrics = make(map[string]T)
}
