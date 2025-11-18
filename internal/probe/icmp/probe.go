package icmp

import (
	"context"
	"strconv"
	"time"

	"github.com/go-ping/ping"
	"github.com/prometheus/client_golang/prometheus"
	klog "k8s.io/klog/v2"

	"gitlab.snapp.ir/snappcloud/health_exporter/internal/config"
	"gitlab.snapp.ir/snappcloud/health_exporter/internal/metrics"
	"gitlab.snapp.ir/snappcloud/health_exporter/internal/probe"
)

type Probe struct {
	target   config.ICMPTarget
	metrics  *metrics.ICMP
	interval time.Duration
}

func New(target config.ICMPTarget, m *metrics.ICMP) *Probe {
	return &Probe{
		target:   target,
		metrics:  m,
		interval: probe.IntervalFromRPS(target.RPS),
	}
}

func (p *Probe) Run(ctx context.Context) error {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			go p.probeOnce()
		}
	}
}

func (p *Probe) probeOnce() {
	stats := p.sendRequest()
	result := "icmp_error"
	if stats.err == nil {
		result = "icmp_success"
	} else {
		klog.V(4).Infof("icmp probe failure for host %s: %v", p.target.Host, stats.err)
	}

	labels := prometheus.Labels{
		"host":   p.target.Host,
		"name":   p.target.Name,
		"ttl":    strconv.Itoa(stats.ttl),
		"result": result,
	}

	p.metrics.Requests.With(labels).Inc()
	p.metrics.Durations.With(labels).Observe(stats.rtt.Seconds())
}

type icmpStats struct {
	rtt time.Duration
	ttl int
	err error
}

func (p *Probe) sendRequest() icmpStats {
	pinger, err := ping.NewPinger(p.target.Host)
	if err != nil {
		return icmpStats{err: err}
	}

	pinger.Count = 1
	pinger.Timeout = p.target.Timeout
	pinger.TTL = p.target.TTL
	pinger.SetPrivileged(false)

	result := icmpStats{ttl: p.target.TTL}
	pinger.OnRecv = func(pkt *ping.Packet) {
		result = icmpStats{
			rtt: pkt.Rtt,
			ttl: pkt.Ttl,
		}
	}

	if err := pinger.Run(); err != nil {
		result.err = err
		return result
	}

	return result
}
