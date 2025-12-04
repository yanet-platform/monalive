package prometheus

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "go.uber.org/zap"

	"github.com/yanet-platform/monalive/internal/monitoring/metrics"
)

type Provider struct {
	registry *prometheus.Registry

	counters   *Registry[prometheus.Counter]
	gauges     *Registry[prometheus.Gauge]
	histograms *Registry[prometheus.Histogram]

	countersVec   *Registry[*CounterVec]
	gaugesVec     *Registry[*GaugeVec]
	histogramsVec *Registry[*HistogramVec]

	log *log.Logger
}

func NewProvider(logger *log.Logger) *Provider {
	registry := prometheus.NewRegistry()
	return &Provider{
		registry:      registry,
		counters:      newRegistry[prometheus.Counter](registry),
		gauges:        newRegistry[prometheus.Gauge](registry),
		histograms:    newRegistry[prometheus.Histogram](registry),
		countersVec:   newRegistry[*CounterVec](registry),
		gaugesVec:     newRegistry[*GaugeVec](registry),
		histogramsVec: newRegistry[*HistogramVec](registry),
		log:           logger.With(log.String("metrics_provider", "prometheus")),
	}
}

func (m *Provider) GetCounter(name string, opts ...metrics.MetricOption) metrics.Counter {
	options := applyOpts(opts)

	counter, err := m.counters.GetOrCreateMetric(name, func() prometheus.Counter {
		return prometheus.NewCounter(
			prometheus.CounterOpts{
				Name:        name,
				Help:        options.Description,
				ConstLabels: prometheus.Labels(options.ConstLabels),
			},
		)
	})
	if err != nil {
		m.log.Error("failed to create counter", log.String("name", name), log.Error(err))
		return &metrics.NopCounter{}
	}

	return counter
}

func (m *Provider) GetGauge(name string, opts ...metrics.MetricOption) metrics.Gauge {
	options := applyOpts(opts)

	gauge, err := m.gauges.GetOrCreateMetric(name, func() prometheus.Gauge {
		return prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:        name,
				Help:        options.Description,
				ConstLabels: prometheus.Labels(options.ConstLabels),
			},
		)
	})
	if err != nil {
		m.log.Error("failed to create gauge", log.String("name", name), log.Error(err))
		return &metrics.NopGauge{}
	}

	return gauge
}

func (m *Provider) GetHistogram(name string, buckets []float64, opts ...metrics.MetricOption) metrics.Histogram {
	options := applyOpts(opts)

	histogram, err := m.histograms.GetOrCreateMetric(name, func() prometheus.Histogram {
		return prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:        name,
				Help:        options.Description,
				Buckets:     buckets,
				ConstLabels: prometheus.Labels(options.ConstLabels),
			},
		)
	})
	if err != nil {
		m.log.Error("failed to create histogram", log.String("name", name), log.Error(err))
		return &metrics.NopHistogram{}
	}

	return histogram
}

func (m *Provider) GetCounterVec(name string, labelNames []string, opts ...metrics.MetricOption) metrics.CounterVec {
	options := applyOpts(opts)

	counterVec, err := m.countersVec.GetOrCreateMetric(name, func() *CounterVec {
		return newCounterVec(
			prometheus.CounterOpts{
				Name:        name,
				Help:        options.Description,
				ConstLabels: prometheus.Labels(options.ConstLabels),
			},
			labelNames,
			m.log,
		)
	})
	if err != nil {
		m.log.Error("failed to create counter vector", log.String("name", name), log.Error(err))
		return &metrics.NopCounterVec{}
	}

	return counterVec

}

func (m *Provider) GetGaugeVec(name string, labelNames []string, opts ...metrics.MetricOption) metrics.GaugeVec {
	options := applyOpts(opts)

	gaugeVec, err := m.gaugesVec.GetOrCreateMetric(name, func() *GaugeVec {
		return newGaugeVec(
			prometheus.GaugeOpts{
				Name:        name,
				Help:        options.Description,
				ConstLabels: prometheus.Labels(options.ConstLabels),
			},
			labelNames,
			m.log,
		)
	})
	if err != nil {
		m.log.Error("failed to create gauge vector", log.String("name", name), log.Error(err))
		return &metrics.NopGaugeVec{}
	}

	return gaugeVec

}

func (m *Provider) GetHistogramVec(name string, buckets []float64, labelNames []string, opts ...metrics.MetricOption) metrics.HistogramVec {
	options := applyOpts(opts)

	histogramVec, err := m.histogramsVec.GetOrCreateMetric(name, func() *HistogramVec {
		return newHistogramVec(
			prometheus.HistogramOpts{
				Name:        name,
				Help:        options.Description,
				Buckets:     buckets,
				ConstLabels: prometheus.Labels(options.ConstLabels),
			},
			labelNames,
			m.log,
		)
	})
	if err != nil {
		m.log.Error("failed to create histogram vector", log.String("name", name), log.Error(err))
		return &metrics.NopHistogramVec{}
	}

	return histogramVec
}

func (m *Provider) UnregisterMetric(metricType metrics.MetricType, name string) {
	switch metricType {
	case metrics.CounterMetric:
		m.counters.DeleteMetric(name)
	case metrics.GaugeMetric:
		m.gauges.DeleteMetric(name)
	case metrics.HistogramMetric:
		m.histograms.DeleteMetric(name)
	case metrics.CounterVecMetric:
		m.countersVec.DeleteMetric(name)
	case metrics.GaugeVecMetric:
		m.gaugesVec.DeleteMetric(name)
	case metrics.HistogramVecMetric:
		m.histogramsVec.DeleteMetric(name)
	default:
		m.log.Error("unknown metric type", log.String("type", metricType.String()))
	}
}

func (m *Provider) GetHTTPHandler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// Shutdown stops the metrics provider and releases resources
func (m *Provider) Shutdown(ctx context.Context) error {
	m.counters.Shutdown()
	m.gauges.Shutdown()
	m.histograms.Shutdown()
	m.countersVec.Shutdown()
	m.gaugesVec.Shutdown()
	m.histogramsVec.Shutdown()

	return nil
}

func applyOpts(opts []metrics.MetricOption) metrics.MetricOpts {
	var options metrics.MetricOpts
	for _, opt := range opts {
		opt(&options)
	}
	return options
}
