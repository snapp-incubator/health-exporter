package prober

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptrace"
	url2 "net/url"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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
)

// TODO: it could be a bad practice!
func init() {
	prometheus.MustRegister(httpRequests)
	prometheus.MustRegister(httpDurations)
	prometheus.MustRegister(dnsLookupDurations)

}

func NewHttp(name string, url string, rps float64, timeout time.Duration, tlsSkipVerify, disableKeepAlives bool, host string) HTTP {
	client := &http.Client{
		Timeout: timeout,
	}
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.DisableKeepAlives = disableKeepAlives
	if tlsSkipVerify {
		customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	client.Transport = customTransport

	return HTTP{
		Name:   name,
		URL:    url,
		RPS:    rps,
		Client: client,
		Host:   host,
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

type HTTP struct {
	Name   string
	URL    string
	RPS    float64
	Host   string
	Client *http.Client
	ticker *time.Ticker
}

func (h *HTTP) Start(ctx context.Context) {

	h.ticker = time.NewTicker(h.calculateInterval())
	defer h.ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Info("Context is done!")
			return
		case <-h.ticker.C:
			go (func() {
				stats := h.sendRequest(ctx)

				result := stats.ErrorType
				if stats.ErrorType == "" {
					if stats.StatusCode >= 400 && stats.StatusCode < 500 {
						result = "http_client_error"
					} else if stats.StatusCode >= 500 {
						result = "http_server_error"
					} else if stats.StatusCode >= 200 && stats.StatusCode < 400 {
						result = "http_success"
					} else {
						result = "http_other_error"
					}
				}
				httpRequests.With(prometheus.Labels{
					"url":         h.URL,
					"status_code": strconv.Itoa(stats.StatusCode),
					"result":      result,
					"name":        h.Name,
				}).Inc()
				httpDurations.With(prometheus.Labels{
					"url":         h.URL,
					"status_code": strconv.Itoa(stats.StatusCode),
					"result":      result,
					"name":        h.Name,
				}).Observe(stats.ResponseTime)
				dnsLookupDurations.With(prometheus.Labels{
					"url":         h.URL,
					"status_code": strconv.Itoa(stats.StatusCode),
					"result":      result,
					"name":        h.Name,
					"dns_error":   stats.DNSError,
				}).Observe(stats.DNSLookupTime)
			})()

		}
	}
}

func (h *HTTP) calculateInterval() time.Duration {
	return time.Duration(1000.0/h.RPS) * time.Millisecond
}

func (h *HTTP) sendRequest(ctx context.Context) HTTPResult {
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
	req, _ := http.NewRequestWithContext(clientTraceCtx, http.MethodGet, h.URL, nil)
	if h.Host != "" {
		req.Host = h.Host
	}
	startTime = time.Now()
	res, err := h.Client.Do(req)
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
func (h *HTTP) errorType(err error) string {
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
