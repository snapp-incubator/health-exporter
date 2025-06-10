package prober

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	url2 "net/url"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker"
	klog "k8s.io/klog/v2"
)

var (
	httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "health_http_requests_total",
			Help: "The number of http requests",
		},
		[]string{"name", "status_code", "result", "url"},
	)
	httpDurations = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "health_http_duration_seconds",
			Help:    "The response time of http requests",
			Buckets: []float64{0.001, 0.005, 0.01, 0.02, 0.03, 0.05, 0.075, 0.1, 0.2, 0.5, 0.75, 1, 1.5, 2, 2.5, 3, 4, 5},
		},
		[]string{"name", "status_code", "result", "url"},
	)
	dnsLookupDurations = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "health_http_dns_lookup_time_seconds",
			Help:    "The response time of dns lookup",
			Buckets: []float64{0.0005, 0.001, 0.002, 0.003, 0.004, 0.005, 0.006, 0.008, 0.01, 0.015, 0.02, 0.03, 0.05, 0.075, 0.1, 0.2, 0.5, 1},
		},
		[]string{"name", "status_code", "dns_error", "result", "url"},
	)

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

// TODO: it could be a bad practice!
func init() {
	prometheus.MustRegister(httpRequests)
	prometheus.MustRegister(httpDurations)
	prometheus.MustRegister(dnsLookupDurations)
	prometheus.MustRegister(httpDuration)
	prometheus.MustRegister(httpErrors)
	prometheus.MustRegister(httpCircuitBreakerState)
}

func NewHttp(name string, url string, rps float64, timeout time.Duration, tlsSkipVerify, disableKeepAlives, h2cEnabled bool, host string) *HttpProber {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     disableKeepAlives,
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
		disableKeepAlive: disableKeepAlives,
		h2cEnabled:       h2cEnabled,
		host:             host,
		client:           client,
		breaker:          breaker,
	}
}

type HTTPResult struct {
	ResponseTime  float64
	StatusCode    int
	Error         error
	ErrorType     string
	DNSLookupTime float64
	DNSError      string
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

func (h *HttpProber) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(float64(time.Second) / h.rps))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go h.probe(ctx)
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

func (h *HttpProber) sendRequest(ctx context.Context) HTTPResult {
	var startTime time.Time
	var dnsStartTime time.Time
	var dnsLookupTime float64
	var dnsError string
	var dnsHost string
	httpTrace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			dnsHost = info.Host
			dnsStartTime = time.Now()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			dnsLookupTime = time.Since(dnsStartTime).Seconds()
			if info.Err != nil {
				klog.Infof("DNS Error for %v in %v seconds: %v ", dnsHost, dnsLookupTime, info.Err)
				dnsError = info.Err.Error()
			}
		},
	}

	clientTraceCtx := httptrace.WithClientTrace(ctx, httpTrace)
	req, _ := http.NewRequestWithContext(clientTraceCtx, http.MethodGet, h.url, nil)
	if h.host != "" {
		req.Host = h.host
	}
	startTime = time.Now()
	res, err := h.client.Do(req)
	responseTime := time.Since(startTime).Seconds()

	if err != nil {
		return HTTPResult{
			ResponseTime:  responseTime,
			Error:         err,
			ErrorType:     h.errorType(err),
			DNSLookupTime: dnsLookupTime,
			DNSError:      dnsError,
		}
	}
	defer res.Body.Close()
	return HTTPResult{
		StatusCode:    res.StatusCode,
		ResponseTime:  responseTime,
		DNSLookupTime: dnsLookupTime,
		DNSError:      dnsError,
	}
}

func (h *HttpProber) errorType(err error) string {
	if timeoutError, ok := err.(net.Error); ok && timeoutError.Timeout() {
		return "timeout"
	}
	urlErr, isUrlErr := err.(*url2.Error)
	if !isUrlErr {
		return "connection_failed"
	}

	opErr, isNetErr := (urlErr.Err).(*net.OpError)
	if isNetErr {
		switch (opErr.Err).(type) {
		case *net.DNSError:
			return "dns_error"
		case *net.ParseError:
			return "address_error"
		}
	}

	return "connection_failed"
}
