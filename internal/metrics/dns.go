package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type DNS struct {
	Requests  *prometheus.CounterVec
	Durations *prometheus.HistogramVec
}

var (
	dnsOnce sync.Once
	dnsInst *DNS
)

func NewDNS(reg prometheus.Registerer) *DNS {
	dnsOnce.Do(func() {
		dnsInst = &DNS{
			Requests: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "health_dns_requests_total",
				Help: "The number of dns requests",
			}, []string{"name", "rcode", "rcode_value", "result", "domain", "server"}),
			Durations: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:    "health_dns_duration_seconds",
				Help:    "The response time of dns requests",
				Buckets: []float64{0.001, 0.005, 0.01, 0.02, 0.03, 0.05, 0.075, 0.1, 0.2, 0.5, 0.75, 1, 1.5, 2, 2.5, 3, 4, 5},
			}, []string{"name", "rcode", "rcode_value", "result", "domain", "server"}),
		}
		reg.MustRegister(dnsInst.Requests, dnsInst.Durations)
	})
	return dnsInst
}
