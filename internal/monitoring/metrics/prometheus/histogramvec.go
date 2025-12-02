package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	log "go.uber.org/zap"

	"github.com/yanet-platform/monalive/internal/monitoring/metrics"
)

// histogramVec is an alias for [prometheus.HistogramVec], existing to make an
// embedded field private
type histogramVec = prometheus.HistogramVec

// HistogramVec is a wrapper for [prometheus.HistogramVec] with a logger for
// error handling
type HistogramVec struct {
	*histogramVec
	log *log.Logger
}

func newHistogramVec(opts prometheus.HistogramOpts, labelNames []string, logger *log.Logger) *HistogramVec {
	return &HistogramVec{
		histogramVec: prometheus.NewHistogramVec(opts, labelNames),
		log:          logger.With(log.String("type", "histogram_vec"), log.String("name", opts.Name)),
	}
}

func (m *HistogramVec) GetMetricWith(labels metrics.Labels) metrics.Histogram {
	histogramVec, err := m.histogramVec.GetMetricWith(prometheus.Labels(labels))
	if err != nil {
		m.log.Error("failed to create metric", log.Error(err))
		return &metrics.NopHistogram{}
	}

	return histogramVec
}

func (m *HistogramVec) CurryWith(labels metrics.Labels) metrics.HistogramVec {
	histogramVec, err := m.histogramVec.CurryWith(prometheus.Labels(labels))
	if err != nil {
		m.log.Error("failed to carry metric", log.Error(err))
		return &metrics.NopHistogramVec{}
	}

	return &HistogramVec{
		histogramVec: histogramVec.(*prometheus.HistogramVec),
		log:          m.log,
	}
}

func (m *HistogramVec) DeletePartialMatch(labels metrics.Labels) {
	m.histogramVec.DeletePartialMatch(prometheus.Labels(labels))
}

func (m *HistogramVec) Delete(labels metrics.Labels) {
	m.histogramVec.Delete(prometheus.Labels(labels))
}
