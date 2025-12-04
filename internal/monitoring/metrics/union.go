package metrics

type CounterUnion []Counter

var _ Counter = CounterUnion{}

func NewCounterUnion(counters ...Counter) CounterUnion {
	return counters
}

func (m CounterUnion) Inc() {
	for _, counter := range m {
		counter.Inc()
	}
}

func (m CounterUnion) Add(val float64) {
	for _, counter := range m {
		counter.Add(val)
	}
}

type GaugeUnion []Gauge

var _ Gauge = GaugeUnion{}

func NewGaugeUnion(gauges ...Gauge) GaugeUnion {
	return gauges
}

func (m GaugeUnion) Add(val float64) {
	for _, gauge := range m {
		gauge.Add(val)
	}
}

func (m GaugeUnion) Sub(val float64) {
	for _, gauge := range m {
		gauge.Sub(val)
	}
}

func (m GaugeUnion) Set(val float64) {
	for _, gauge := range m {
		gauge.Set(val)
	}
}

type HistogramUnion []Histogram

var _ Histogram = HistogramUnion{}

func NewHistogramUnion(histograms ...Histogram) HistogramUnion {
	return histograms
}

func (m HistogramUnion) Observe(val float64) {
	for _, histogram := range m {
		histogram.Observe(val)
	}
}

type CounterVecUnion []CounterVec

var _ CounterVec = CounterVecUnion{}

func NewCounterVecUnion(counterVecs ...CounterVec) CounterVecUnion {
	return counterVecs
}

func (m CounterVecUnion) GetMetricWith(labels Labels) Counter {
	counterUnion := make(CounterUnion, len(m))
	for i, counterVec := range m {
		counterUnion[i] = counterVec.GetMetricWith(labels)
	}
	return counterUnion
}

func (m CounterVecUnion) CurryWith(labels Labels) CounterVec {
	counterVecUnion := make(CounterVecUnion, len(m))
	for i, counterVec := range m {
		counterVecUnion[i] = counterVec.CurryWith(labels)
	}
	return counterVecUnion
}

func (m CounterVecUnion) Delete(labels Labels) {
	for _, counterVec := range m {
		counterVec.Delete(labels)
	}
}

func (m CounterVecUnion) DeletePartialMatch(labels Labels) {
	for _, counterVec := range m {
		counterVec.DeletePartialMatch(labels)
	}
}

type GaugeVecUnion []GaugeVec

var _ GaugeVec = GaugeVecUnion{}

func NewGaugeVecUnion(gaugeVecs ...GaugeVec) GaugeVecUnion {
	return gaugeVecs
}

func (m GaugeVecUnion) GetMetricWith(labels Labels) Gauge {
	gaugeUnion := make(GaugeUnion, len(m))
	for i, gaugeVec := range m {
		gaugeUnion[i] = gaugeVec.GetMetricWith(labels)
	}
	return gaugeUnion
}

func (m GaugeVecUnion) CurryWith(labels Labels) GaugeVec {
	gaugeVecUnion := make(GaugeVecUnion, len(m))
	for i, gaugeVec := range m {
		gaugeVecUnion[i] = gaugeVec.CurryWith(labels)
	}
	return gaugeVecUnion
}

func (m GaugeVecUnion) Delete(labels Labels) {
	for _, gaugeVec := range m {
		gaugeVec.Delete(labels)
	}
}

func (m GaugeVecUnion) DeletePartialMatch(labels Labels) {
	for _, gaugeVec := range m {
		gaugeVec.DeletePartialMatch(labels)
	}
}

type HistogramVecUnion []HistogramVec

var _ HistogramVec = HistogramVecUnion{}

func NewHistogramVecUnion(histogramVecs ...HistogramVec) HistogramVecUnion {
	return histogramVecs
}

func (m HistogramVecUnion) GetMetricWith(labels Labels) Histogram {
	histogramUnion := make(HistogramUnion, len(m))
	for i, histogramVec := range m {
		histogramUnion[i] = histogramVec.GetMetricWith(labels)
	}
	return histogramUnion
}

func (m HistogramVecUnion) CurryWith(labels Labels) HistogramVec {
	histogramVecUnion := make(HistogramVecUnion, len(m))
	for i, histogramVec := range m {
		histogramVecUnion[i] = histogramVec.CurryWith(labels)
	}
	return histogramVecUnion
}

func (m HistogramVecUnion) Delete(labels Labels) {
	for _, histogramVec := range m {
		histogramVec.Delete(labels)
	}
}

func (m HistogramVecUnion) DeletePartialMatch(labels Labels) {
	for _, histogramVec := range m {
		histogramVec.DeletePartialMatch(labels)
	}
}
