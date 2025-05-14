package prober

import (
	"context"
	"net"
	"time"

	"gitlab.snapp.ir/snappcloud/health_exporter/metrics"
	"k8s.io/klog/v2"
)

type Dns struct {
	name     string
	server   string
	domain   string
	interval int
}

func NewDns(name, server, domain string, interval int) *Dns {
	return &Dns{name: name, server: server, domain: domain, interval: interval}
}

func (d *Dns) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(d.interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Infof("[DNS] Stopping probe '%s'", d.name)
			return
		case <-ticker.C:
			d.probe()
		}
	}
}

func (d *Dns) probe() {
	start := time.Now()
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("udp", d.server)
		},
	}

	_, err := resolver.LookupHost(context.Background(), d.domain)
	duration := time.Since(start).Seconds()

	status := "success"
	if err != nil {
		klog.Warningf("[DNS] %s failed: %v", d.name, err)
		status = "failed"
	}
	metrics.DNSLookupDuration.WithLabelValues(d.name, d.domain, status).Observe(duration)
}
