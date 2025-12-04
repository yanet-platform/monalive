package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	log "go.uber.org/zap"

	"github.com/yanet-platform/monalive/internal/monitoring/metrics"
)

// gaugeVec is an alias for [prometheus.GaugeVec], existing to make an embedded
// field private
type gaugeVec = prometheus.GaugeVec

// GaugeVec is a wrapper for [prometheus.GaugeVec] with a logger for error
// handling
type GaugeVec struct {
	*gaugeVec
	log *log.Logger
}

func newGaugeVec(opts prometheus.GaugeOpts, labelNames []string, logger *log.Logger) *GaugeVec {
	return &GaugeVec{
		gaugeVec: prometheus.NewGaugeVec(opts, labelNames),
		log:      logger.With(log.String("type", "gauge_vec"), log.String("name", opts.Name)),
	}
}

func (m *GaugeVec) GetMetricWith(labels metrics.Labels) metrics.Gauge {
	gauge, err := m.gaugeVec.GetMetricWith(prometheus.Labels(labels))
	if err != nil {
		m.log.Error("failed to create metric", log.Error(err))
		return &metrics.NopGauge{}
	}

	return gauge
}

func (m *GaugeVec) CurryWith(labels metrics.Labels) metrics.GaugeVec {
	gaugeVec, err := m.gaugeVec.CurryWith(prometheus.Labels(labels))
	if err != nil {
		m.log.Error("failed to carry metric", log.Error(err))
		return &metrics.NopGaugeVec{}
	}

	return &GaugeVec{
		gaugeVec: gaugeVec,
		log:      m.log,
	}
}

func (m *GaugeVec) DeletePartialMatch(labels metrics.Labels) {
	m.gaugeVec.DeletePartialMatch(prometheus.Labels(labels))
}

func (m *GaugeVec) Delete(labels metrics.Labels) {
	m.gaugeVec.Delete(prometheus.Labels(labels))
}
