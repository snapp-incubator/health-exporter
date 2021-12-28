package main

import (
	"context"
	"flag"
	"net/http"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	klog "k8s.io/klog/v2"

	"gitlab.snapp.ir/snappcloud/health_exporter/config"
	"gitlab.snapp.ir/snappcloud/health_exporter/prober"
)

var configPath string

func init() {
	flag.StringVar(&configPath, "config", "config.yaml", "Path of config file")
	flag.Parse()
}

func main() {

	err := config.Read(configPath)

	if err != nil {
		klog.Fatalf("Cannot read/parse config file: %v", err)
	}

	klog.Infof("Using config file '%s'\n", configPath)

	ctx, cancel := context.WithCancel(context.Background())

	for _, ht := range config.Get().Targets.HTTP {
		httpProber := prober.NewHttp(ht.Name, ht.URL, ht.RPS, ht.Timeout, ht.TLSSkipVerify, ht.DisableKeepAlives, ht.Host)

		klog.Infof("Probing HTTP target '%s' with url '%s', RPS: %.2f, timeout: %s, TLS_skip_verify: %v, disableKeepAlives: %v ...\n",
			ht.Name, ht.URL, ht.RPS, ht.Timeout, ht.TLSSkipVerify, ht.DisableKeepAlives)
		go httpProber.Start(ctx)
	}

	for _, d := range config.Get().Targets.DNS {
		if d.ServerIP == "" {
			config, _ := dns.ClientConfigFromFile("/etc/resolv.conf")
			d.ServerIP = config.Servers[0]
		}
		if d.ServerPort == 0 {
			d.ServerPort = 53
		}
		dnsProber := prober.NewDNS(d.Name, d.Domain, d.RecordType, d.RPS, d.ServerIP, d.ServerPort, d.Timeout)

		klog.Infof("Probing DNS target '%s' with domain '%s', RecordType: %s, RPS: %.2f, server: %s, port: %v, timeout: %s ...\n",
			d.Name, d.Domain, d.RecordType, d.RPS, d.ServerIP, d.ServerPort, d.Timeout)
		go dnsProber.Start(ctx)
	}
	if config.Get().Targets.K8S.Enabled {
		klog.Infof("K8S Prober is Enabled")
		k8s_client := prober.Getk8sClient()
		for _, sp := range config.Get().Targets.K8S.SimpleProbe {
			k8s_simpeProber := prober.NewSimpleProbe(k8s_client, sp.NameSpace, sp.RPS)
			klog.Infof("Probing K8S target namespace '%s', RPS: 1.0 ...\n",
				sp.NameSpace)
			go k8s_simpeProber.Start(ctx)
		}
	} else {
		klog.Infof("K8S Prober is Disabled")
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	startServer(config.Get().Listen, mux, cancel)
}
