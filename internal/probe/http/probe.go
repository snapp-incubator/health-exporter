package http

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptrace"
	url2 "net/url"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/http2"
	klog "k8s.io/klog/v2"

	"gitlab.snapp.ir/snappcloud/health_exporter/internal/config"
	"gitlab.snapp.ir/snappcloud/health_exporter/internal/metrics"
	"gitlab.snapp.ir/snappcloud/health_exporter/internal/probe"
)

type Probe struct {
	target   config.HTTPTarget
	client   *http.Client
	metrics  *metrics.HTTP
	interval time.Duration
}

func New(target config.HTTPTarget, m *metrics.HTTP) *Probe {
	return &Probe{
		target:   target,
		client:   buildClient(target),
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
			go p.probeOnce(ctx)
		}
	}
}

func (p *Probe) probeOnce(ctx context.Context) {
	stats := p.performRequest(ctx)
	result := stats.resultLabel
	if result == "" {
		result = classifyStatus(stats.statusCode)
	}

	labels := prometheus.Labels{
		"url":         p.target.URL,
		"name":        p.target.Name,
		"status_code": strconv.Itoa(stats.statusCode),
		"result":      result,
	}

	p.metrics.Requests.With(labels).Inc()
	p.metrics.Durations.With(labels).Observe(stats.responseTime)
	p.metrics.DNSLookupTime.With(prometheus.Labels{
		"url":         p.target.URL,
		"name":        p.target.Name,
		"status_code": strconv.Itoa(stats.statusCode),
		"result":      result,
		"dns_error":   stats.dnsError,
	}).Observe(stats.dnsLookup)
}

type httpProbeStats struct {
	statusCode   int
	responseTime float64
	resultLabel  string
	dnsLookup    float64
	dnsError     string
}

func (p *Probe) performRequest(ctx context.Context) httpProbeStats {
	var dnsStart time.Time
	var dnsLookup float64
	var dnsErr string

	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			if !dnsStart.IsZero() {
				dnsLookup = time.Since(dnsStart).Seconds()
			}
			if info.Err != nil {
				dnsErr = info.Err.Error()
				klog.Infof("dns lookup error for %s: %v", p.target.URL, info.Err)
			}
		},
	}

	reqCtx := httptrace.WithClientTrace(ctx, trace)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, p.target.URL, nil)
	if err != nil {
		return httpProbeStats{
			resultLabel: "request_build_error",
		}
	}

	if p.target.Host != "" {
		req.Host = p.target.Host
	}

	start := time.Now()
	resp, err := p.client.Do(req)
	responseTime := time.Since(start).Seconds()

	if err != nil {
		return httpProbeStats{
			responseTime: responseTime,
			resultLabel:  classifyError(err),
			dnsLookup:    dnsLookup,
			dnsError:     dnsErr,
		}
	}
	defer resp.Body.Close()

	return httpProbeStats{
		statusCode:   resp.StatusCode,
		responseTime: responseTime,
		dnsLookup:    dnsLookup,
		dnsError:     dnsErr,
	}
}

func classifyStatus(code int) string {
	switch {
	case code >= 200 && code < 400:
		return "http_success"
	case code >= 400 && code < 500:
		return "http_client_error"
	case code >= 500:
		return "http_server_error"
	default:
		return "http_other_error"
	}
}

func classifyError(err error) string {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}

	var urlErr *url2.Error
	if errors.As(err, &urlErr) {
		var dnsErr *net.DNSError
		if errors.As(urlErr.Err, &dnsErr) {
			return "dns_error"
		}
		var parseErr *net.ParseError
		if errors.As(urlErr.Err, &parseErr) {
			return "address_error"
		}
	}

	return "connection_failed"
}

func buildClient(target config.HTTPTarget) *http.Client {
	client := &http.Client{
		Timeout: target.Timeout,
	}

	if target.H2cEnabled {
		client.Transport = buildHTTP2Transport(target)
		return client
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = target.DisableKeepAlives
	if target.TLSSkipVerify {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = true
	}
	client.Transport = transport
	return client
}

func buildHTTP2Transport(target config.HTTPTarget) http.RoundTripper {
	t := &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, _ *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}
	if target.TLSSkipVerify {
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return t
}
