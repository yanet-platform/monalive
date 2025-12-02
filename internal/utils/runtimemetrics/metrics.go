package runtimemetrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var runtimeRegistry = prometheus.NewRegistry()

func init() {
	runtimeRegistry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	runtimeRegistry.MustRegister(collectors.NewGoCollector())
}

var HTTPHandler = promhttp.HandlerFor(runtimeRegistry, promhttp.HandlerOpts{})
