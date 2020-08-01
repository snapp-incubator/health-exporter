package prober

import (
	"context"
	"crypto/tls"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"net"
	"net/http"
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
)

// TODO: it could be a bad practice!
func init() {
	prometheus.MustRegister(httpRequests)
	prometheus.MustRegister(httpDurations)
}

func NewHttp(name string, url string, rps float64, timeout time.Duration, tlsSkipVerify bool) HTTP {
	client := &http.Client{
		Timeout: timeout,
	}

	if tlsSkipVerify == true {
		customTransport := http.DefaultTransport.(*http.Transport).Clone()
		customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

		client.Transport = customTransport
	}

	return HTTP{
		Name:   name,
		URL:    url,
		RPS:    rps,
		Client: client,
	}
}

type HTTPResult struct {
	ResponseTime float64
	StatusCode   int
	Error        error
	ErrorType    string
}

type HTTP struct {
	Name   string
	URL    string
	RPS    float64
	Client *http.Client
	ticker *time.Ticker
}

func (h *HTTP) Start(ctx context.Context) {

	h.ticker = time.NewTicker(h.calculateInterval())
	defer h.ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Print("Context is done!")
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
			})()

		}
	}
}

func (h *HTTP) calculateInterval() time.Duration {
	return time.Duration(1000.0/h.RPS) * time.Millisecond
}

func (h *HTTP) sendRequest(ctx context.Context) HTTPResult {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, h.URL, nil)

	start := time.Now()
	res, err := h.Client.Do(req)
	responseTime := time.Since(start).Seconds()

	if err != nil {
		return HTTPResult{
			ResponseTime: responseTime,
			Error:        err,
			ErrorType:    h.errorType(err),
		}
	}

	defer res.Body.Close()

	return HTTPResult{
		StatusCode:   res.StatusCode,
		ResponseTime: responseTime,
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
