package metrics

import (
	"context"
	"fmt"
	"net/http"
)

type Metrics interface {
	Provider
	Gatherer
}

type Provider interface {
	GetCounter(name string, opts ...MetricOption) Counter
	GetGauge(name string, opts ...MetricOption) Gauge
	GetHistogram(name string, buckets []float64, opts ...MetricOption) Histogram

	GetCounterVec(name string, labelNames []string, opts ...MetricOption) CounterVec
	GetGaugeVec(name string, labelNames []string, opts ...MetricOption) GaugeVec
	GetHistogramVec(name string, buckets []float64, labelNames []string, opts ...MetricOption) HistogramVec

	UnregisterMetric(metricType MetricType, name string)
	Shutdown(ctx context.Context) error
}

type Gatherer interface {
	GetHTTPHandler() http.Handler
}

type Counter interface {
	Inc()
	Add(float64)
}

type Gauge interface {
	Add(float64)
	Sub(float64)
	Set(float64)
}

type Histogram interface {
	Observe(float64)
}

type Labels map[string]string

type CounterVec interface {
	GetMetricWith(Labels) Counter
	CurryWith(Labels) CounterVec
	Delete(Labels)
	DeletePartialMatch(Labels)
}

type GaugeVec interface {
	GetMetricWith(Labels) Gauge
	CurryWith(Labels) GaugeVec
	Delete(Labels)
	DeletePartialMatch(Labels)
}

type HistogramVec interface {
	GetMetricWith(Labels) Histogram
	CurryWith(Labels) HistogramVec
	Delete(Labels)
	DeletePartialMatch(Labels)
}

type MetricType int

const (
	CounterMetric MetricType = iota + 1
	GaugeMetric
	HistogramMetric
	CounterVecMetric
	GaugeVecMetric
	HistogramVecMetric
)

func (m MetricType) String() string {
	switch m {
	case CounterMetric:
		return "counter"
	case GaugeMetric:
		return "gauge"
	case HistogramMetric:
		return "histogram"
	case CounterVecMetric:
		return "counter_vec"
	case GaugeVecMetric:
		return "gauge_vec"
	case HistogramVecMetric:
		return "histogram_vec"
	default:
		return fmt.Sprintf("unknown(%d)", m)
	}
}
