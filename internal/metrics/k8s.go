package metrics

import (
	"context"
	"net/url"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/metrics"
)

type K8S struct {
	PodCount *prometheus.GaugeVec
}

var (
	clientGoRequestResultMetricVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "health_k8s_http_request_total",
			Help: "Total number of HTTP requests to the Kubernetes API by status code.",
		},
		[]string{"status_code"},
	)
	clientGoRequestLatencyMetricVec = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "health_k8s_http_request_duration_seconds",
			Help:       "Summary of latencies for HTTP requests to the Kubernetes API by endpoint.",
			Objectives: map[float64]float64{},
		},
		[]string{"endpoint"},
	)
	registerClientMetricsOnce sync.Once
	k8sMetricsOnce            sync.Once
	k8sMetricsInst            *K8S
)

type clientGoRequestMetricAdapter struct{}

func (clientGoRequestMetricAdapter) Increment(_ context.Context, code string, _ string, _ string) {
	clientGoRequestResultMetricVec.WithLabelValues(code).Inc()
}

func (clientGoRequestMetricAdapter) Observe(_ context.Context, _ string, u url.URL, latency time.Duration) {
	clientGoRequestLatencyMetricVec.WithLabelValues(u.EscapedPath()).Observe(latency.Seconds())
}

func RegisterClientGoMetrics(reg prometheus.Registerer) {
	registerClientMetricsOnce.Do(func() {
		metrics.Register(metrics.RegisterOpts{
			RequestLatency: clientGoRequestMetricAdapter{},
			RequestResult:  clientGoRequestMetricAdapter{},
		})
		reg.MustRegister(
			clientGoRequestResultMetricVec,
			clientGoRequestLatencyMetricVec,
		)
	})
}

func NewK8S(reg prometheus.Registerer) *K8S {
	k8sMetricsOnce.Do(func() {
		k8sMetricsInst = &K8S{
			PodCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: "health_k8s_pod_count",
				Help: "The number of pods in namespace",
			}, []string{"namespace"}),
		}
		reg.MustRegister(k8sMetricsInst.PodCount)
	})
	return k8sMetricsInst
}
