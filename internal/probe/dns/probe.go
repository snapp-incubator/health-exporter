package dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	klog "k8s.io/klog/v2"

	"gitlab.snapp.ir/snappcloud/health_exporter/internal/config"
	"gitlab.snapp.ir/snappcloud/health_exporter/internal/metrics"
	"gitlab.snapp.ir/snappcloud/health_exporter/internal/probe"
)

type Probe struct {
	target   config.DNSTarget
	client   *dns.Client
	metrics  *metrics.DNS
	interval time.Duration
	server   string
}

func New(target config.DNSTarget, m *metrics.DNS) *Probe {
	return &Probe{
		target:   target,
		client:   &dns.Client{Timeout: target.Timeout},
		metrics:  m,
		interval: probe.IntervalFromRPS(target.RPS),
		server:   net.JoinHostPort(target.ServerIP, strconv.Itoa(target.ServerPort)),
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
	stats := p.sendRequest(ctx)
	labels := prometheus.Labels{
		"domain":      p.target.Domain,
		"rcode":       stats.rcode,
		"rcode_value": strconv.Itoa(stats.rcodeValue),
		"result":      stats.result,
		"name":        p.target.Name,
		"server":      p.server,
	}

	p.metrics.Requests.With(labels).Inc()
	p.metrics.Durations.With(labels).Observe(stats.responseTime)
}

type dnsProbeStats struct {
	responseTime float64
	rcode        string
	rcodeValue   int
	result       string
}

func (p *Probe) sendRequest(ctx context.Context) dnsProbeStats {
	msg := new(dns.Msg)
	recordType, err := p.parseRecordType()
	if err != nil {
		klog.Warning(err)
		return dnsProbeStats{
			rcodeValue: -1,
			result:     "error",
		}
	}
	msg.SetQuestion(dns.Fqdn(p.target.Domain), recordType)
	msg.RecursionDesired = true

	resp, rtt, err := p.client.ExchangeContext(ctx, msg, p.server)
	responseTime := float64(rtt) / float64(time.Second)
	if err != nil {
		return dnsProbeStats{
			rcodeValue:   -1,
			result:       classifyDNSError(err),
			responseTime: responseTime,
		}
	}
	if resp.Rcode != dns.RcodeSuccess {
		return dnsProbeStats{
			rcodeValue: resp.Rcode,
			rcode:      dns.RcodeToString[resp.Rcode],
			result:     "error",
		}
	}
	return dnsProbeStats{
		responseTime: responseTime,
		rcodeValue:   resp.Rcode,
		rcode:        dns.RcodeToString[resp.Rcode],
		result:       "success",
	}
}

func (p *Probe) parseRecordType() (uint16, error) {
	switch strings.ToUpper(p.target.RecordType) {
	case "A":
		return dns.TypeA, nil
	case "AAAA":
		return dns.TypeAAAA, nil
	case "ANY":
		return dns.TypeANY, nil
	case "CNAME":
		return dns.TypeCNAME, nil
	case "MX":
		return dns.TypeMX, nil
	case "NS":
		return dns.TypeNS, nil
	case "PTR":
		return dns.TypePTR, nil
	case "SOA":
		return dns.TypeSOA, nil
	case "SPF":
		return dns.TypeSPF, nil
	case "SRV":
		return dns.TypeSRV, nil
	case "TXT":
		return dns.TypeTXT, nil
	default:
		return 0, fmt.Errorf("record type %s not recognized", p.target.RecordType)
	}
}

func classifyDNSError(err error) string {
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Timeout() {
		return "timeout"
	}
	return "error"
}
