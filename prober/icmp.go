package prober

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus-community/pro-bing"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker"
	klog "k8s.io/klog/v2"
)

var (
	icmpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "icmp_probe_duration_seconds",
			Help:    "Duration of ICMP probe in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"name", "host", "status"},
	)

	icmpErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "icmp_probe_errors_total",
			Help: "Total number of ICMP probe errors",
		},
		[]string{"name", "host", "error_type"},
	)

	icmpCircuitBreakerState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "icmp_probe_circuit_breaker_state",
			Help: "Current state of the circuit breaker (0: closed, 1: half-open, 2: open)",
		},
		[]string{"name", "host"},
	)

	icmpPacketLoss = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "icmp_probe_packet_loss_percent",
			Help: "Percentage of packet loss in ICMP probe",
		},
		[]string{"name", "host"},
	)

	icmpRTT = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "icmp_probe_rtt_seconds",
			Help:    "Round-trip time of ICMP probe in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"name", "host"},
	)
)

func init() {
	prometheus.MustRegister(icmpDuration)
	prometheus.MustRegister(icmpErrors)
	prometheus.MustRegister(icmpCircuitBreakerState)
	prometheus.MustRegister(icmpPacketLoss)
	prometheus.MustRegister(icmpRTT)
}

type ICMPProber struct {
	name    string
	host    string
	rps     float64
	ttl     int
	timeout time.Duration
	breaker *gobreaker.CircuitBreaker
}

func NewICMP(name, host string, rps float64, ttl int, timeout time.Duration) *ICMPProber {
	breaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        fmt.Sprintf("%s-%s", name, host),
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
			icmpCircuitBreakerState.WithLabelValues(name, host).Set(float64(state))
		},
	})

	return &ICMPProber{
		name:    name,
		host:    host,
		rps:     rps,
		ttl:     ttl,
		timeout: timeout,
		breaker: breaker,
	}
}

func (p *ICMPProber) Start(ctx context.Context) {
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

func (p *ICMPProber) probe(ctx context.Context) {
	start := time.Now()

	_, err := p.breaker.Execute(func() (interface{}, error) {
		pinger, err := pro_bing.NewPinger(p.host)
		if err != nil {
			return nil, fmt.Errorf("failed to create pinger: %v", err)
		}

		pinger.SetPrivileged(true)
		pinger.Count = 1
		pinger.Timeout = p.timeout
		pinger.TTL = p.ttl

		if err := pinger.RunWithContext(ctx); err != nil {
			return nil, fmt.Errorf("failed to run pinger: %v", err)
		}

		stats := pinger.Statistics()
		if stats.PacketsRecv == 0 {
			return nil, fmt.Errorf("no packets received")
		}

		// Record packet loss
		icmpPacketLoss.WithLabelValues(p.name, p.host).Set(stats.PacketLoss)

		// Record RTT if we have valid measurements
		if stats.AvgRtt > 0 {
			icmpRTT.WithLabelValues(p.name, p.host).Observe(stats.AvgRtt.Seconds())
		}

		return stats, nil
	})

	duration := time.Since(start).Seconds()
	status := "success"
	if err != nil {
		status = "error"
		icmpErrors.WithLabelValues(p.name, p.host, "request_failed").Inc()
		klog.Errorf("ICMP probe failed for %s: %v", p.name, err)
	}

	icmpDuration.WithLabelValues(p.name, p.host, status).Observe(duration)
}
