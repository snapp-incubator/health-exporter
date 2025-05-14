package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	HTTPResponseDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "health_exporter_http_response_duration_seconds",
			Help:    "HTTP probe response durations by status",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"name", "url", "status"},
	)

	DNSLookupDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "health_exporter_dns_lookup_duration_seconds",
			Help:    "DNS lookup durations by result",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"name", "domain", "status"},
	)

	ICMPDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "health_exporter_icmp_probe_duration_seconds",
			Help:    "ICMP round-trip times",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"name", "host", "status"},
	)

	K8SResponseDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "health_exporter_k8s_pod_list_duration_seconds",
			Help:    "Kubernetes pod listing durations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"namespace", "label_selector", "status"},
	)
)

func init() {
	prometheus.MustRegister(
		HTTPResponseDuration,
		DNSLookupDuration,
		ICMPDuration,
		K8SResponseDuration,
	)
}
