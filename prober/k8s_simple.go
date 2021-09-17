// the Simple Probe will register pod count of a namespace as prometheus metric
// the purpose is to test client_metrics module which stores response time metric

package prober

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	klog "k8s.io/klog/v2"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	podCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "health_k8s_pod_count",
			Help: "The number of pods in namespace",
		},
		[]string{"namespace"},
	)
)

// TODO: it could be a bad practice!
func init() {
	prometheus.MustRegister(podCount)
}

type SimpleProbe struct {
	Client    *kubernetes.Clientset
	NameSpace string
	RPS       float64
	ticker    *time.Ticker
}

func NewSimpleProbe(client *kubernetes.Clientset, namespace string, rps float64) SimpleProbe {
	return SimpleProbe{
		Client:    client,
		NameSpace: namespace,
		RPS:       rps,
	}
}

func (sp *SimpleProbe) Start(ctx context.Context) {
	sp.ticker = time.NewTicker(sp.calculateInterval())
	defer sp.ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Info("Context is done!")
			return
		case <-sp.ticker.C:
			go (func() {
				sp.GetPods(ctx)

			})()
		}
	}
}

func (sp *SimpleProbe) GetPods(ctx context.Context) {
	pods, err := sp.Client.CoreV1().Pods(sp.NameSpace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Warning(err)
	}
	pod_count := float64(len(pods.Items))
	podCount.With(prometheus.Labels{
		"namespace": sp.NameSpace,
	}).Set(pod_count)
}

func (sp *SimpleProbe) calculateInterval() time.Duration {
	return time.Duration(1000.0/sp.RPS) * time.Millisecond
}
