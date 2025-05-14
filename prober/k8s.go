package prober

import (
	"context"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"gitlab.snapp.ir/snappcloud/health_exporter/metrics"
)

type K8s struct {
	namespace     string
	labelSelector string
	interval      int
	clientset     *kubernetes.Clientset
}

func NewK8s(namespace, labelSelector string, interval int) *K8s {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("[K8S] Error creating cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("[K8S] Error creating Kubernetes client: %v", err)
	}

	return &K8s{
		namespace:     namespace,
		labelSelector: labelSelector,
		interval:      interval,
		clientset:     clientset,
	}
}

func (k *K8s) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(k.interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Infof("[K8S] Stopping Kubernetes probe")
			return
		case <-ticker.C:
			k.probe()
		}
	}
}

func (k *K8s) probe() {
	start := time.Now()

	_, err := k.clientset.CoreV1().Pods(k.namespace).List(context.Background(), v1.ListOptions{
		LabelSelector: k.labelSelector,
	})
	duration := time.Since(start).Seconds()

	status := "success"
	if err != nil {
		klog.Warningf("[K8S] Probe failed: %v", err)
		status = "failed"
	}

	metrics.K8SResponseDuration.WithLabelValues(k.namespace, k.labelSelector, status).Observe(duration)
}
