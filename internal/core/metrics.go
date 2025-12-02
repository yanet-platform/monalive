package core

import (
	"github.com/yanet-platform/monalive/internal/monitoring/metrics"
	"github.com/yanet-platform/monalive/internal/types/key"
)

// Metrics ...
type Metrics struct {
	realsEnabled           metrics.Gauge
	realsEnabledPerService metrics.GaugeVec

	reals           metrics.Gauge
	realsPerService metrics.GaugeVec

	realsTrasitionPeriod           metrics.Histogram
	realsTrasitionPeriodPerService metrics.HistogramVec

	realsResponseTime           metrics.Histogram
	realsResponseTimePerService metrics.HistogramVec

	realsErrors           metrics.CounterVec
	realsErrorsPerService metrics.CounterVec
}

// NewMetrics ...
func NewMetrics(provider *metrics.ScopedMetrics) *Metrics {
	var dummyServiceKey key.Service
	serviceLabelNames := dummyServiceKey.LabelNames()
	return &Metrics{
		realsEnabled: provider.Scope(metrics.Global).GetGauge(
			"reals_enabled",
			metrics.WithDescription("number of enabled reals"),
		),
		realsEnabledPerService: provider.Scope(metrics.PerService).GetGaugeVec(
			"reals_enabled",
			serviceLabelNames,
			metrics.WithDescription("number of enabled reals for service"),
		),

		reals: provider.Scope(metrics.Global).GetGauge(
			"reals",
			metrics.WithDescription("number of reals"),
		),
		realsPerService: provider.Scope(metrics.PerService).GetGaugeVec(
			"reals",
			serviceLabelNames,
			metrics.WithDescription("number of reals for service"),
		),

		realsTrasitionPeriod: provider.Scope(metrics.Global).GetHistogram(
			"reals_transitions",
			[]float64{10, 30, 60, 180},
			metrics.WithDescription("observe reals transition period"),
		),
		realsTrasitionPeriodPerService: provider.Scope(metrics.PerService).GetHistogramVec(
			"reals_transitions",
			[]float64{10, 30, 60, 180},
			serviceLabelNames,
			metrics.WithDescription("observe reals transition period for service"),
		),

		realsResponseTime: provider.Scope(metrics.Global).GetHistogram(
			"reals_response_duration",
			[]float64{0.01, 0.05, 0.1, 0.15, 0.3, 0.5, 1},
			metrics.WithDescription("observe reals response time"),
		),
		realsResponseTimePerService: provider.Scope(metrics.PerService).GetHistogramVec(
			"reals_response_duration",
			[]float64{0.01, 0.05, 0.1, 0.15, 0.3, 0.5, 1},
			serviceLabelNames,
			metrics.WithDescription("observe reals response time for service"),
		),

		realsErrors: provider.Scope(metrics.Global).GetCounterVec(
			"reals_error",
			[]string{"error"},
			metrics.WithDescription("observe reals errors"),
		),
		realsErrorsPerService: provider.Scope(metrics.PerService).GetCounterVec(
			"reals_error",
			append(serviceLabelNames, "error"),
			metrics.WithDescription("observe reals errors for service"),
		),
	}
}

func (m *Metrics) RealsEnabled() metrics.Gauge {
	return m.realsEnabled
}

func (m *Metrics) RealsEnabledForService(serviceLabels metrics.Labels) metrics.Gauge {
	return metrics.NewGaugeUnion(
		m.realsEnabledPerService.GetMetricWith(serviceLabels),
		m.realsEnabled,
	)
}

func (m *Metrics) Reals() metrics.Gauge {
	return m.reals
}

func (m *Metrics) RealsForService(serviceLabels metrics.Labels) metrics.Gauge {
	return metrics.NewGaugeUnion(
		m.realsPerService.GetMetricWith(serviceLabels),
		m.reals,
	)
}

func (m *Metrics) RealsTrasitionPeriodForService(serviceLabels metrics.Labels) metrics.Histogram {
	return metrics.NewHistogramUnion(
		m.realsTrasitionPeriodPerService.GetMetricWith(serviceLabels),
		m.realsTrasitionPeriod,
	)
}

func (m *Metrics) RealsResponseTimeForService(serviceLabels metrics.Labels) metrics.Histogram {
	return metrics.NewHistogramUnion(
		m.realsResponseTimePerService.GetMetricWith(serviceLabels),
		m.realsResponseTime,
	)
}

func (m *Metrics) RealsErrorsForService(serviceLabels metrics.Labels) metrics.CounterVec {
	return metrics.NewCounterVecUnion(
		m.realsErrorsPerService.CurryWith(serviceLabels),
		m.realsErrors,
	)
}

func (m *Metrics) DeleteService(serviceLabels metrics.Labels) {
	m.realsEnabledPerService.Delete(serviceLabels)
	m.realsPerService.Delete(serviceLabels)
	m.realsTrasitionPeriodPerService.Delete(serviceLabels)
	m.realsResponseTimePerService.Delete(serviceLabels)
	m.realsErrorsPerService.DeletePartialMatch(serviceLabels)
}
