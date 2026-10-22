package announcer

import "github.com/yanet-platform/monalive/internal/monitoring/metrics"

type Metrics struct {
	prefixPerGroup          metrics.GaugeVec
	announcedPrefixPerGroup metrics.GaugeVec
}

func NewMetrics(provider metrics.Provider) *Metrics {
	return &Metrics{
		prefixPerGroup: provider.GetGaugeVec("vip", []string{"announce_group"},
			metrics.WithDescription("number of VIPs per announce group")),
		announcedPrefixPerGroup: provider.GetGaugeVec("vip_announced", []string{"announce_group"},
			metrics.WithDescription("number of announced VIPs per announce group")),
	}
}

func groupLabels(group string) metrics.Labels {
	return metrics.Labels{"announce_group": group}
}

func (m *Metrics) PrefixesPerGroup() metrics.GaugeVec {
	return m.prefixPerGroup
}

func (m *Metrics) AnnouncedPrefixes(group string) metrics.Gauge {
	return m.announcedPrefixPerGroup.GetMetricWith(groupLabels(group))
}
