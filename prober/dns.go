package prober

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker"
	klog "k8s.io/klog/v2"
)

var (
	dnsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dns_probe_duration_seconds",
			Help:    "Duration of DNS probe in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"name", "domain", "record_type", "status"},
	)

	dnsErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dns_probe_errors_total",
			Help: "Total number of DNS probe errors",
		},
		[]string{"name", "domain", "record_type", "error_type"},
	)

	dnsCircuitBreakerState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dns_probe_circuit_breaker_state",
			Help: "Current state of the circuit breaker (0: closed, 1: half-open, 2: open)",
		},
		[]string{"name", "domain"},
	)

	dnsResponseCode = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "dns_probe_response_code",
			Help: "DNS response code (0: NoError, 1: FormErr, 2: ServFail, 3: NXDomain, etc.)",
		},
		[]string{"name", "domain", "record_type"},
	)
)

func init() {
	prometheus.MustRegister(dnsDuration)
	prometheus.MustRegister(dnsErrors)
	prometheus.MustRegister(dnsCircuitBreakerState)
	prometheus.MustRegister(dnsResponseCode)
}

type DNSProber struct {
	name       string
	domain     string
	recordType string
	rps        float64
	serverIP   string
	serverPort int
	timeout    time.Duration
	client     *dns.Client
	breaker    *gobreaker.CircuitBreaker
}

func NewDNS(name, domain, recordType string, rps float64, serverIP string, serverPort int, timeout time.Duration) *DNSProber {
	client := &dns.Client{
		Net:     "udp",
		Timeout: timeout,
	}

	breaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        fmt.Sprintf("%s-%s", name, domain),
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
			dnsCircuitBreakerState.WithLabelValues(name, domain).Set(float64(state))
		},
	})

	return &DNSProber{
		name:       name,
		domain:     domain,
		recordType: recordType,
		rps:        rps,
		serverIP:   serverIP,
		serverPort: serverPort,
		timeout:    timeout,
		client:     client,
		breaker:    breaker,
	}
}

func (p *DNSProber) Start(ctx context.Context) {
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

func (p *DNSProber) probe(ctx context.Context) {
	start := time.Now()

	msg := new(dns.Msg)
	msg.SetQuestion(p.domain, dns.StringToType[p.recordType])
	msg.RecursionDesired = true

	_, err := p.breaker.Execute(func() (interface{}, error) {
		resp, _, err := p.client.Exchange(msg, net.JoinHostPort(p.serverIP, strconv.Itoa(p.serverPort)))
		if err != nil {
			return nil, fmt.Errorf("DNS query failed: %v", err)
		}

		// Record response code
		dnsResponseCode.WithLabelValues(p.name, p.domain, p.recordType).Set(float64(resp.Rcode))

		if resp.Rcode != dns.RcodeSuccess {
			return nil, fmt.Errorf("DNS error: %s", dns.RcodeToString[resp.Rcode])
		}

		return resp, nil
	})

	duration := time.Since(start).Seconds()
	status := "success"
	if err != nil {
		status = "error"
		dnsErrors.WithLabelValues(p.name, p.domain, p.recordType, "request_failed").Inc()
		klog.Errorf("DNS probe failed for %s: %v", p.name, err)
	}

	dnsDuration.WithLabelValues(p.name, p.domain, p.recordType, status).Observe(duration)
}
