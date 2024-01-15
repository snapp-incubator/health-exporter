package prober

import (
	"context"
	"strconv"
	"time"

	"github.com/go-ping/ping"
	"github.com/prometheus/client_golang/prometheus"
	klog "k8s.io/klog/v2"
)

var (
	icmpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "health_icmp_requests_total",
			Help: "The number of icmp requests",
		},
		[]string{"name", "ttl", "result", "host"},
	)
	icmpDurations = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "health_icmp_duration_seconds",
			Help:    "The response time of icmp requests",
			Buckets: []float64{0.001, 0.005, 0.01, 0.02, 0.03, 0.05, 0.075, 0.1, 0.2, 0.5, 0.75, 1, 1.5, 2, 2.5, 3, 4, 5},
		},
		[]string{"name", "ttl", "result", "host"},
	)
)

type ICMP struct {
	Name    string
	Host    string
	TTL     int
	RPS     float64
	Timeout time.Duration
	ticker  *time.Ticker
}
type ICMPResult struct {
	RTT   time.Duration
	TTL   int
	Error error
}

// TODO: it could be a bad practice!
func init() {
	prometheus.MustRegister(icmpDurations)
	prometheus.MustRegister(icmpRequests)

}

func NewICMP(name string, host string, rps float64, ttl int, timeout time.Duration) ICMP {

	return ICMP{
		Name:    name,
		Host:    host,
		TTL:     ttl,
		RPS:     rps,
		Timeout: timeout,
	}

}

func (i *ICMP) sendRequest(ctx context.Context) ICMPResult {
	icmpresult := ICMPResult{}
	pinger, err := ping.NewPinger(i.Host)
	if err != nil {
		return ICMPResult{Error: err}
	}
	//set default
	pinger.Timeout = time.Second * 100000
	pinger.TTL = 64
	pinger.Timeout = i.Timeout
	if i.TTL != 0 {
		pinger.TTL = i.TTL
	}
	pinger.SetPrivileged(false)
	pinger.OnRecv = func(pkt *ping.Packet) {
		icmpresult = ICMPResult{
			RTT: pkt.Rtt,
			TTL: pkt.Ttl,
		}
	}
	err = pinger.Run()
	if err != nil {
		return ICMPResult{Error: err}
	}
	return icmpresult

}

func (i *ICMP) Start(ctx context.Context) {
	i.ticker = time.NewTicker(calculateInterval(i.RPS))
	defer i.ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Info("Context is done!")
			return
		case <-i.ticker.C:
			go (func() {
				stats := i.sendRequest(ctx)
				var result string
				if stats.Error == nil {
					result = "icmp_success"
				} else {
					result = "icmp_error"
				}

				icmpRequests.With(prometheus.Labels{
					"host":   i.Host,
					"result": result,
					"name":   i.Name,
					"ttl":    strconv.Itoa(stats.TTL),
				}).Inc()
				icmpDurations.With(prometheus.Labels{
					"host":   i.Host,
					"result": result,
					"name":   i.Name,
					"ttl":    strconv.Itoa(stats.TTL),
				}).Observe(float64(stats.RTT))
			})()

		}
	}
}
