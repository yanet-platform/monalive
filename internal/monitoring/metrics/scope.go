package metrics

import (
	"context"
	"sync"

	log "go.uber.org/zap"
)

type Scope string

const (
	Global     Scope = "global"
	PerService Scope = "per_service"
)

type ScopedMetrics struct {
	scopes map[Scope]Metrics
	mu     sync.RWMutex

	zero Metrics

	log *log.Logger
}

func NewScopedMetrics(logger *log.Logger) *ScopedMetrics {
	return &ScopedMetrics{
		scopes: make(map[Scope]Metrics),
		zero:   &NopMetrics{},
	}
}

func (m *ScopedMetrics) Scope(scope Scope) Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if metrics, exists := m.scopes[scope]; exists {
		return metrics
	}
	return m.zero
}

func (m *ScopedMetrics) Set(scope Scope, metrics Metrics) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scopes[scope] = metrics
}

func (m *ScopedMetrics) Shutdown(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for scope, metrics := range m.scopes {
		if err := metrics.Shutdown(ctx); err != nil {
			m.log.Error(
				"failed to shutdown metrics",
				log.String("scope", string(scope)),
				log.Error(err),
			)
		}
	}

	m.scopes = make(map[Scope]Metrics)
}

func (m *ScopedMetrics) Gatherers() map[Scope]Gatherer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	gatherers := make(map[Scope]Gatherer, len(m.scopes))
	for scope, metrics := range m.scopes {
		gatherers[scope] = metrics
	}

	return gatherers
}
