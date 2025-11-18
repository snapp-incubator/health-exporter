package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type HTTP struct {
	Requests      *prometheus.CounterVec
	Durations     *prometheus.HistogramVec
	DNSLookupTime *prometheus.HistogramVec
}

var (
	httpOnce sync.Once
	httpInst *HTTP
)

func NewHTTP(reg prometheus.Registerer) *HTTP {
	httpOnce.Do(func() {
		httpInst = &HTTP{
			Requests: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "health_http_requests_total",
				Help: "The number of http requests",
			}, []string{"name", "status_code", "result", "url"}),
			Durations: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:    "health_http_duration_seconds",
				Help:    "The response time of http requests",
				Buckets: []float64{0.001, 0.005, 0.01, 0.02, 0.03, 0.05, 0.075, 0.1, 0.2, 0.5, 0.75, 1, 1.5, 2, 2.5, 3, 4, 5},
			}, []string{"name", "status_code", "result", "url"}),
			DNSLookupTime: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:    "health_http_dns_lookup_time_seconds",
				Help:    "The response time of dns lookup",
				Buckets: []float64{0.0005, 0.001, 0.002, 0.003, 0.004, 0.005, 0.006, 0.008, 0.01, 0.015, 0.02, 0.03, 0.05, 0.075, 0.1, 0.2, 0.5, 1},
			}, []string{"name", "status_code", "dns_error", "result", "url"}),
		}
		reg.MustRegister(httpInst.Requests, httpInst.Durations, httpInst.DNSLookupTime)
	})
	return httpInst
}
