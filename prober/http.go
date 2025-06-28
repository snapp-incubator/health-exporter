package prober

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"gitlab.snapp.ir/snappcloud/health_exporter/metrics"
	"k8s.io/klog/v2"
)

type Http struct {
	name              string
	url               string
	rps               int
	timeout           time.Duration
	tlsSkipVerify     bool
	disableKeepAlives bool
	h2cEnabled        bool
	host              string
	client            *http.Client
}

func NewHttp(name, url string, rps int, timeout time.Duration, tlsSkipVerify, disableKeepAlives, h2cEnabled bool, host string) *Http {
	transport := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: tlsSkipVerify}, // nolint: gosec
		DisableKeepAlives: disableKeepAlives,
	}

	return &Http{
		name:              name,
		url:               url,
		rps:               rps,
		timeout:           timeout,
		tlsSkipVerify:     tlsSkipVerify,
		disableKeepAlives: disableKeepAlives,
		h2cEnabled:        h2cEnabled,
		host:              host,
		client: &http.Client{
			Timeout:   time.Duration(timeout) * time.Second,
			Transport: transport,
		},
	}
}

func (h *Http) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Second / time.Duration(h.rps))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Infof("[HTTP] Shutting down probe '%s'", h.name)
			return
		case <-ticker.C:
			h.probe()
		}
	}
}

func (h *Http) probe() {
	start := time.Now()

	req, err := http.NewRequest("GET", h.url, nil)
	if err != nil {
		klog.Errorf("[HTTP] %s - request creation error: %v", h.name, err)
		return
	}

	if h.host != "" {
		req.Host = h.host
		req.Header.Set("Host", h.host)
	}

	resp, err := h.client.Do(req)
	duration := time.Since(start).Seconds()

	if err != nil {
		klog.Warningf("[HTTP] %s - request failed: %v", h.name, err)
		metrics.HTTPResponseDuration.WithLabelValues(h.name, h.url, "failed").Observe(duration)
		return
	}
	defer resp.Body.Close()

	metrics.HTTPResponseDuration.WithLabelValues(h.name, h.url, resp.Status).Observe(duration)
}
