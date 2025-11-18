package k8s

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	klog "k8s.io/klog/v2"

	"gitlab.snapp.ir/snappcloud/health_exporter/internal/config"
	"gitlab.snapp.ir/snappcloud/health_exporter/internal/metrics"
	"gitlab.snapp.ir/snappcloud/health_exporter/internal/probe"
)

type SimpleProbe struct {
	client    *kubernetes.Clientset
	metrics   *metrics.K8S
	interval  time.Duration
	namespace string
}

func NewSimpleProbe(client *kubernetes.Clientset, target config.K8SSimpleProbe, m *metrics.K8S) *SimpleProbe {
	return &SimpleProbe{
		client:    client,
		metrics:   m,
		interval:  probe.IntervalFromRPS(target.RPS),
		namespace: target.NameSpace,
	}
}

func (s *SimpleProbe) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			go s.poll(ctx)
		}
	}
}

func (s *SimpleProbe) poll(ctx context.Context) {
	pods, err := s.client.CoreV1().Pods(s.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Warningf("k8s probe namespace %s: %v", s.namespace, err)
		return
	}
	s.metrics.PodCount.With(prometheus.Labels{
		"namespace": s.namespace,
	}).Set(float64(len(pods.Items)))
}
