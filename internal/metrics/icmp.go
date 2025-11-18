package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type ICMP struct {
	Requests  *prometheus.CounterVec
	Durations *prometheus.HistogramVec
}

var (
	icmpOnce sync.Once
	icmpInst *ICMP
)

func NewICMP(reg prometheus.Registerer) *ICMP {
	icmpOnce.Do(func() {
		icmpInst = &ICMP{
			Requests: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "health_icmp_requests_total",
				Help: "The number of icmp requests",
			}, []string{"name", "ttl", "result", "host"}),
			Durations: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:    "health_icmp_duration_seconds",
				Help:    "The response time of icmp requests",
				Buckets: []float64{0.001, 0.005, 0.01, 0.02, 0.03, 0.05, 0.075, 0.1, 0.2, 0.5, 0.75, 1, 1.5, 2, 2.5, 3, 4, 5},
			}, []string{"name", "ttl", "result", "host"}),
		}
		reg.MustRegister(icmpInst.Requests, icmpInst.Durations)
	})
	return icmpInst
}
