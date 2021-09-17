package prober

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	klog "k8s.io/klog/v2"
)

var (
	dnsRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "health_dns_requests_total",
			Help: "The number of dns requests",
		},
		[]string{"name", "rcode", "rcode_value", "result", "domain", "server"},
	)
	dnsDurations = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "health_dns_duration_seconds",
			Help:    "The response time of dns requests",
			Buckets: []float64{0.001, 0.005, 0.01, 0.02, 0.03, 0.05, 0.075, 0.1, 0.2, 0.5, 0.75, 1, 1.5, 2, 2.5, 3, 4, 5},
		},
		[]string{"name", "rcode", "rcode_value", "result", "domain", "server"},
	)
)

// TODO: it could be a bad practice!
func init() {
	prometheus.MustRegister(dnsRequests)
	prometheus.MustRegister(dnsDurations)
}

func NewDNS(name string, domain string, recordType string, rps float64, serverIP string, serverPort int, timeout time.Duration) DNS {
	c := &dns.Client{
		Timeout: timeout,
	}

	return DNS{
		Name:       name,
		Domain:     domain,
		RecordType: recordType,
		RPS:        rps,
		ServerIP:   serverIP,
		ServerPort: serverPort,
		Client:     c,
	}
}

type DNSResult struct {
	ResponseTime float64
	RCode        string
	RCodeValue   int
	Result       string
	Error        error
}

type DNS struct {
	Name       string
	Domain     string
	RPS        float64
	RecordType string
	ServerIP   string
	ServerPort int
	Client     *dns.Client
	ticker     *time.Ticker
}

func (d *DNS) Start(ctx context.Context) {

	d.ticker = time.NewTicker(d.calculateInterval())
	defer d.ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Info("Context is done!")
			return
		case <-d.ticker.C:
			go (func() {
				stats := d.sendRequest(ctx)

				dnsRequests.With(prometheus.Labels{
					"domain":      d.Domain,
					"rcode":       stats.RCode,
					"rcode_value": strconv.Itoa(stats.RCodeValue),
					"result":      stats.Result,
					"name":        d.Name,
					"server":      net.JoinHostPort(d.ServerIP, strconv.Itoa(d.ServerPort)),
				}).Inc()
				dnsDurations.With(prometheus.Labels{
					"domain":      d.Domain,
					"rcode":       stats.RCode,
					"rcode_value": strconv.Itoa(stats.RCodeValue),
					"result":      stats.Result,
					"name":        d.Name,
					"server":      net.JoinHostPort(d.ServerIP, strconv.Itoa(d.ServerPort)),
				}).Observe(stats.ResponseTime)
			})()

		}
	}
}

func (d *DNS) calculateInterval() time.Duration {
	return time.Duration(1000.0/d.RPS) * time.Millisecond
}

func (d *DNS) sendRequest(ctx context.Context) DNSResult {
	m := new(dns.Msg)
	recordType, err := d.parseRecordType()
	if err != nil {
		return DNSResult{
			ResponseTime: 0,
			RCodeValue:   -1,
			Error:        err,
			Result:       "error",
		}
	}
	m.SetQuestion(dns.Fqdn(d.Domain), recordType)
	m.RecursionDesired = true

	r, rtt, err := d.Client.ExchangeContext(ctx, m, net.JoinHostPort(d.ServerIP, strconv.Itoa(d.ServerPort)))
	ResponseTime := float64(rtt.Milliseconds()) / 1e3
	if err != nil {
		return DNSResult{
			ResponseTime: 0,
			RCodeValue:   -1,
			Result:       d.errorType(err),
			Error:        err,
		}
	} else if r.Rcode != dns.RcodeSuccess {
		return DNSResult{
			ResponseTime: 0,
			RCodeValue:   r.Rcode,
			RCode:        dns.RcodeToString[r.Rcode],
			Result:       "error",
			Error:        fmt.Errorf("invalid answer (%s) after %s query for %s", dns.RcodeToString[r.Rcode], d.RecordType, d.Domain),
		}
	} else {
		return DNSResult{
			ResponseTime: ResponseTime,
			RCodeValue:   r.Rcode,
			RCode:        dns.RcodeToString[r.Rcode],
			Result:       "success",
			Error:        err,
		}
	}
}

func (d *DNS) errorType(err error) string {
	if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
		return "timeout"
	}
	return "error"
}

func (d *DNS) parseRecordType() (uint16, error) {
	var recordType uint16
	var err error

	switch d.RecordType {
	case "A":
		recordType = dns.TypeA
	case "AAAA":
		recordType = dns.TypeAAAA
	case "ANY":
		recordType = dns.TypeANY
	case "CNAME":
		recordType = dns.TypeCNAME
	case "MX":
		recordType = dns.TypeMX
	case "NS":
		recordType = dns.TypeNS
	case "PTR":
		recordType = dns.TypePTR
	case "SOA":
		recordType = dns.TypeSOA
	case "SPF":
		recordType = dns.TypeSPF
	case "SRV":
		recordType = dns.TypeSRV
	case "TXT":
		recordType = dns.TypeTXT
	default:
		err = fmt.Errorf("Record type %s not recognized", d.RecordType)
	}

	return recordType, err
}
