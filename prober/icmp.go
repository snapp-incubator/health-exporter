package prober

import (
	"context"
	"time"

	"github.com/go-ping/ping"
	"gitlab.snapp.ir/snappcloud/health_exporter/metrics"
	"k8s.io/klog/v2"
)

type Icmp struct {
	name     string
	host     string
	interval int
}

func NewIcmp(name, host string, interval int) *Icmp {
	return &Icmp{name: name, host: host, interval: interval}
}

func (i *Icmp) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(i.interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Infof("[ICMP] Stopping probe '%s'", i.name)
			return
		case <-ticker.C:
			i.probe()
		}
	}
}

func (i *Icmp) probe() {
	pinger, err := ping.NewPinger(i.host)
	if err != nil {
		klog.Errorf("[ICMP] %s - cannot create pinger: %v", i.name, err)
		return
	}
	pinger.Count = 1
	pinger.Timeout = time.Second * 2

	err = pinger.Run()
	stats := pinger.Statistics()
	duration := stats.AvgRtt.Seconds()

	status := "success"
	if err != nil || stats.PacketsRecv == 0 {
		klog.Warningf("[ICMP] %s - ping failed", i.name)
		status = "failed"
	}
	metrics.ICMPDuration.WithLabelValues(i.name, i.host, status).Observe(duration)
}
