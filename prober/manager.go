package prober

import (
	"context"
	"k8s.io/klog/v2"

	"gitlab.snapp.ir/snappcloud/health_exporter/config"
)

func StartAll(ctx context.Context, targets config.Target) {
	for _, ht := range targets.HTTP {
		httpProber := NewHttp(ht.Name, ht.URL, ht.RPS, ht.Timeout, ht.TLSSkipVerify, ht.DisableKeepAlives, ht.H2cEnabled, ht.Host)
		go httpProber.Start(ctx)
		klog.Infof("[HTTP] Started probe for '%s' -> %s", ht.Name, ht.URL)
	}

	for _, dns := range targets.DNS {
		dnsProber := NewDns(dns.Name, dns.Server, dns.Domain, dns.Interval)
		go dnsProber.Start(ctx)
		klog.Infof("[DNS] Started probe for '%s' on server %s", dns.Name, dns.Server)
	}

	for _, icmp := range targets.ICMP {
		icmpProber := NewIcmp(icmp.Name, icmp.Host, icmp.Interval)
		go icmpProber.Start(ctx)
		klog.Infof("[ICMP] Started probe for '%s' -> %s", icmp.Name, icmp.Host)
	}

	if targets.K8S.Enabled {
		k8sProber := NewK8s(targets.K8S.Namespace, targets.K8S.LabelSelector, targets.K8S.Interval)
		go k8sProber.Start(ctx)
		klog.Infof("[K8S] Started probe for pods in namespace '%s'", targets.K8S.Namespace)
	}
}
