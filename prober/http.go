package prober

import (
	"context"
	"crypto/tls"
	"github.com/prometheus/client_golang/prometheus"
	klog "k8s.io/klog/v2"
	"net"
	"net/http"
	"net/http/httptrace"
	url2 "net/url"
	"strconv"
	"time"
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
			Buckets: []float64{0.001, 0.005, 0.01, 0.02, 0.03, 0.05, 0.075, 0.1, 0.2, 0.5, 0.75, 1, 1.5, 2, 2.5, 3, 4, 5},
		},
		[]string{"name", "status_code", "result", "url"},
	)
)

// TODO: it could be a bad practice!
func init() {
	prometheus.MustRegister(httpRequests)
	prometheus.MustRegister(httpDurations)
	prometheus.MustRegister(dnsLookupDurations)

}

func NewHttp(name string, url string, rps float64, timeout time.Duration, tlsSkipVerify bool, host string) HTTP {
	client := &http.Client{
		Timeout: timeout,
	}
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.DisableKeepAlives = true
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
	dnsLookupTime float64
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
				}).Observe(stats.dnsLookupTime)
			})()

		}
	}
}

func (h *HTTP) calculateInterval() time.Duration {
	return time.Duration(1000.0/h.RPS) * time.Millisecond
}

func (h *HTTP) sendRequest(ctx context.Context) HTTPResult {
	var startTime time.Time
	var finishTime time.Time
	var dnsStartTime time.Time
	var dnsDoneTime time.Time
	var connectStartTime time.Time
	var connectDoneTime time.Time
	var tlsHandshakeStartTime time.Time
	var tlsHandshakeDoneTime time.Time
	httpTrace := &httptrace.ClientTrace{
		GetConn: func(_ string) {
		},
		GotConn: func(info httptrace.GotConnInfo) {
		},
		DNSStart: func(_ httptrace.DNSStartInfo) {
			dnsStartTime = time.Now()
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			dnsDoneTime = time.Now()
		},
		ConnectStart: func(_, _ string) {
			connectStartTime = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			connectDoneTime = time.Now()
		},
		TLSHandshakeStart: func() {
			tlsHandshakeStartTime = time.Now()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			tlsHandshakeDoneTime = time.Now()
		},
	}

	req, _ := http.NewRequest(http.MethodGet, h.URL, nil)
	clientTraceCtx := httptrace.WithClientTrace(req.Context(), httpTrace)
	req = req.WithContext(clientTraceCtx)

	startTime = time.Now()
	res, err := h.Client.Do(req)
	finishTime = time.Now()

	responseTime := finishTime.Sub(startTime)
	dnsLookupTime := dnsDoneTime.Sub(dnsStartTime)


	if err != nil {
		return HTTPResult{
			ResponseTime:  float64(responseTime),
			Error:         err,
			ErrorType:     h.errorType(err),
			dnsLookupTime: float64(dnsLookupTime),
		}
	}
	defer res.Body.Close()
	return HTTPResult{
		StatusCode:    res.StatusCode,
		ResponseTime:  float64(responseTime),
		dnsLookupTime: float64(dnsLookupTime),
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
