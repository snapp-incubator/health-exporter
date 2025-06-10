// the Simple Probe will register pod count of a namespace as prometheus metric
// the purpose is to test client_metrics module which stores response time metric

package prober

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	klog "k8s.io/klog/v2"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker"
)

var (
	k8sDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "k8s_probe_duration_seconds",
			Help:    "Duration of Kubernetes probe in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"name", "namespace", "status"},
	)

	k8sErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "k8s_probe_errors_total",
			Help: "Total number of Kubernetes probe errors",
		},
		[]string{"name", "namespace", "error_type"},
	)

	k8sCircuitBreakerState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "k8s_probe_circuit_breaker_state",
			Help: "Current state of the circuit breaker (0: closed, 1: half-open, 2: open)",
		},
		[]string{"name", "namespace"},
	)

	k8sResourceCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "k8s_probe_resource_count",
			Help: "Number of resources in the namespace",
		},
		[]string{"name", "namespace", "resource_type"},
	)
)

// TODO: it could be a bad practice!
func init() {
	prometheus.MustRegister(k8sDuration)
	prometheus.MustRegister(k8sErrors)
	prometheus.MustRegister(k8sCircuitBreakerState)
	prometheus.MustRegister(k8sResourceCount)
}

type K8SProber struct {
	name      string
	namespace string
	rps       float64
	client    *kubernetes.Clientset
	breaker   *gobreaker.CircuitBreaker
}

func NewSimpleProbe(client *kubernetes.Clientset, namespace string, rps float64) *K8SProber {
	breaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        fmt.Sprintf("k8s-%s", namespace),
		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 5 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			klog.Infof("Circuit breaker '%s' changed from %v to %v", name, from, to)
			state := 0
			switch to {
			case gobreaker.StateHalfOpen:
				state = 1
			case gobreaker.StateOpen:
				state = 2
			}
			k8sCircuitBreakerState.WithLabelValues(name, namespace).Set(float64(state))
		},
	})

	return &K8SProber{
		name:      "k8s",
		namespace: namespace,
		rps:       rps,
		client:    client,
		breaker:   breaker,
	}
}

func (p *K8SProber) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(float64(time.Second) / p.rps))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go p.probe(ctx)
		}
	}
}

func (p *K8SProber) probe(ctx context.Context) {
	start := time.Now()

	_, err := p.breaker.Execute(func() (interface{}, error) {
		// Get pods
		pods, err := p.client.CoreV1().Pods(p.namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list pods: %v", err)
		}
		k8sResourceCount.WithLabelValues(p.name, p.namespace, "pods").Set(float64(len(pods.Items)))

		// Get services
		services, err := p.client.CoreV1().Services(p.namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list services: %v", err)
		}
		k8sResourceCount.WithLabelValues(p.name, p.namespace, "services").Set(float64(len(services.Items)))

		// Get deployments
		deployments, err := p.client.AppsV1().Deployments(p.namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list deployments: %v", err)
		}
		k8sResourceCount.WithLabelValues(p.name, p.namespace, "deployments").Set(float64(len(deployments.Items)))

		return nil, nil
	})

	duration := time.Since(start).Seconds()
	status := "success"
	if err != nil {
		status = "error"
		k8sErrors.WithLabelValues(p.name, p.namespace, "request_failed").Inc()
		klog.Errorf("K8S probe failed for namespace %s: %v", p.namespace, err)
	}

	k8sDuration.WithLabelValues(p.name, p.namespace, status).Observe(duration)
}
