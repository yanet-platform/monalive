package metrics

import (
	"context"
	"net/http"
)

type NopMetrics struct {
	NopProvider
	NopGatherer
}

var _ Metrics = &NopMetrics{}

type NopProvider struct{}

var _ Provider = &NopProvider{}

func (p *NopProvider) GetCounter(_ string, _ ...MetricOption) Counter {
	return &NopCounter{}
}

func (p *NopProvider) GetGauge(_ string, _ ...MetricOption) Gauge {
	return &NopGauge{}
}

func (p *NopProvider) GetHistogram(_ string, _ []float64, _ ...MetricOption) Histogram {
	return &NopHistogram{}
}

func (p *NopProvider) GetCounterVec(_ string, _ []string, _ ...MetricOption) CounterVec {
	return &NopCounterVec{}
}

func (p *NopProvider) GetGaugeVec(_ string, _ []string, _ ...MetricOption) GaugeVec {
	return &NopGaugeVec{}
}

func (p *NopProvider) GetHistogramVec(_ string, _ []float64, _ []string, _ ...MetricOption) HistogramVec {
	return &NopHistogramVec{}
}

func (p *NopProvider) UnregisterMetric(_ MetricType, _ string) {}

func (p *NopProvider) Shutdown(_ context.Context) error {
	return nil
}

type NopCounter struct{}

var _ Counter = &NopCounter{}

func (c *NopCounter) Inc() {}

func (c *NopCounter) Add(_ float64) {}

type NopGauge struct{}

var _ Gauge = &NopGauge{}

func (g *NopGauge) Set(_ float64) {}

func (g *NopGauge) Add(_ float64) {}

func (g *NopGauge) Sub(_ float64) {}

type NopHistogram struct{}

var _ Histogram = &NopHistogram{}

func (h *NopHistogram) Observe(_ float64) {}

type NopCounterVec struct{}

var _ CounterVec = &NopCounterVec{}

func (cv *NopCounterVec) GetMetricWith(_ Labels) Counter {
	return &NopCounter{}
}

func (cv *NopCounterVec) CurryWith(_ Labels) CounterVec {
	return &NopCounterVec{}
}

func (cv *NopCounterVec) Delete(_ Labels) {}

func (cv *NopCounterVec) DeletePartialMatch(_ Labels) {}

type NopGaugeVec struct{}

var _ GaugeVec = &NopGaugeVec{}

func (gv *NopGaugeVec) GetMetricWith(_ Labels) Gauge {
	return &NopGauge{}
}

func (gv *NopGaugeVec) CurryWith(_ Labels) GaugeVec {
	return &NopGaugeVec{}
}

func (gv *NopGaugeVec) Delete(_ Labels) {}

func (gv *NopGaugeVec) DeletePartialMatch(_ Labels) {}

type NopHistogramVec struct{}

var _ HistogramVec = &NopHistogramVec{}

func (hv *NopHistogramVec) GetMetricWith(_ Labels) Histogram {
	return &NopHistogram{}
}

func (hv *NopHistogramVec) CurryWith(_ Labels) HistogramVec {
	return &NopHistogramVec{}
}

func (hv *NopHistogramVec) Delete(_ Labels) {}

func (hv *NopHistogramVec) DeletePartialMatch(_ Labels) {}

type NopGatherer struct{}

var _ Gatherer = &NopGatherer{}

func (g *NopGatherer) GetHTTPHandler() http.Handler {
	return http.NotFoundHandler()
}
