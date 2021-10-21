// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prober

import (
	"context"
	"net/url"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/metrics"
)

var (
	// Metrics for client-go's HTTP requests.
	clientGoRequestResultMetricVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			// Namespace: metricsNamespace,
			Name: "health_k8s_http_request_total",
			Help: "Total number of HTTP requests to the Kubernetes API by status code.",
		},
		[]string{"status_code"},
	)
	clientGoRequestLatencyMetricVec = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			// Namespace:  metricsNamespace,
			Name:       "health_k8s_http_request_duration_seconds",
			Help:       "Summary of latencies for HTTP requests to the Kubernetes API by endpoint.",
			Objectives: map[float64]float64{},
		},
		[]string{"endpoint"},
	)
)

// Definition of client-go metrics adapters for HTTP requests observation
type clientGoRequestMetricAdapter struct{}

func (f *clientGoRequestMetricAdapter) Register(registerer prometheus.Registerer) {
	metrics.Register(
		metrics.RegisterOpts{
			RequestLatency: f,
			RequestResult:  f,
		},
	)
	registerer.MustRegister(
		clientGoRequestResultMetricVec,
		clientGoRequestLatencyMetricVec,
	)
}
func (clientGoRequestMetricAdapter) Increment(ctx context.Context, code string, method string, host string) {
	clientGoRequestResultMetricVec.WithLabelValues(code).Inc()
}
func (clientGoRequestMetricAdapter) Observe(ctx context.Context, verb string, u url.URL, latency time.Duration) {
	clientGoRequestLatencyMetricVec.WithLabelValues(u.EscapedPath()).Observe(latency.Seconds())
}

func init() {
	(&clientGoRequestMetricAdapter{}).Register(prometheus.DefaultRegisterer)
}

func Getk8sClient() *kubernetes.Clientset {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientset
}
