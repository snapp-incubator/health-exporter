package prober

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker"
	klog "k8s.io/klog/v2"
)

var (
	httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_probe_duration_seconds",
			Help:    "Duration of HTTP probe in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"name", "url", "status"},
	)

	httpErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_probe_errors_total",
			Help: "Total number of HTTP probe errors",
		},
		[]string{"name", "url", "error_type"},
	)

	httpCircuitBreakerState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_probe_circuit_breaker_state",
			Help: "Current state of the circuit breaker (0: closed, 1: half-open, 2: open)",
		},
		[]string{"name", "url"},
	)
)

func init() {
	prometheus.MustRegister(httpDuration)
	prometheus.MustRegister(httpErrors)
	prometheus.MustRegister(httpCircuitBreakerState)
}

type HttpProber struct {
	name             string
	url              string
	rps              float64
	timeout          time.Duration
	tlsSkipVerify    bool
	disableKeepAlive bool
	h2cEnabled       bool
	host             string
	client           *http.Client
	breaker          *gobreaker.CircuitBreaker
}

func NewHttp(name, url string, rps float64, timeout time.Duration, tlsSkipVerify, disableKeepAlive, h2cEnabled bool, host string) *HttpProber {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     disableKeepAlive,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: tlsSkipVerify,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	breaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        fmt.Sprintf("%s-%s", name, url),
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
			httpCircuitBreakerState.WithLabelValues(name, url).Set(float64(state))
		},
	})

	return &HttpProber{
		name:             name,
		url:              url,
		rps:              rps,
		timeout:          timeout,
		tlsSkipVerify:    tlsSkipVerify,
		disableKeepAlive: disableKeepAlive,
		h2cEnabled:       h2cEnabled,
		host:             host,
		client:           client,
		breaker:          breaker,
	}
}

func (p *HttpProber) Start(ctx context.Context) {
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

func (p *HttpProber) probe(ctx context.Context) {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, "GET", p.url, nil)
	if err != nil {
		klog.Errorf("Failed to create request for %s: %v", p.name, err)
		httpErrors.WithLabelValues(p.name, p.url, "request_creation").Inc()
		return
	}

	if p.host != "" {
		req.Host = p.host
	}

	_, err = p.breaker.Execute(func() (interface{}, error) {
		resp, err := p.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
		}

		return resp, nil
	})

	duration := time.Since(start).Seconds()
	status := "success"
	if err != nil {
		status = "error"
		httpErrors.WithLabelValues(p.name, p.url, "request_failed").Inc()
		klog.Errorf("HTTP probe failed for %s: %v", p.name, err)
	}

	httpDuration.WithLabelValues(p.name, p.url, status).Observe(duration)
}
