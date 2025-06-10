package main

import (
	"context"
	"flag"
	"net/http"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	klog "k8s.io/klog/v2"

	"gitlab.snapp.ir/snappcloud/health_exporter/config"
	"gitlab.snapp.ir/snappcloud/health_exporter/prober"
)

var (
	configPath string
	version    = "dev"
)

func init() {
	flag.StringVar(&configPath, "config", "config.yaml", "Path of config file")
	flag.Parse()
}

func main() {
	klog.Infof("Starting health-exporter version %s", version)

	if err := config.Read(configPath); err != nil {
		klog.Fatalf("Cannot read/parse config file: %v", err)
	}
	klog.Infof("Using config file '%s'", configPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a WaitGroup to track all running probes
	var wg sync.WaitGroup

	// Start HTTP probes
	for _, ht := range config.Get().Targets.HTTP {
		httpProber := prober.NewHttp(ht.Name, ht.URL, ht.RPS, ht.Timeout, ht.TLSSkipVerify, ht.DisableKeepAlives, ht.H2cEnabled, ht.Host)
		klog.Infof("Starting HTTP probe '%s' with url '%s', RPS: %.2f, timeout: %s, TLS_skip_verify: %v, disableKeepAlives: %v, h2cEnabled: %v",
			ht.Name, ht.URL, ht.RPS, ht.Timeout, ht.TLSSkipVerify, ht.DisableKeepAlives, ht.H2cEnabled)
		
		wg.Add(1)
		go func() {
			defer wg.Done()
			httpProber.Start(ctx)
		}()
	}

	// Start DNS probes
	for _, d := range config.Get().Targets.DNS {
		if d.ServerIP == "" {
			config, err := dns.ClientConfigFromFile("/etc/resolv.conf")
			if err != nil {
				klog.Warningf("Failed to read resolv.conf: %v, using default DNS server", err)
				d.ServerIP = "8.8.8.8"
			} else {
				d.ServerIP = config.Servers[0]
			}
		}
		if d.ServerPort == 0 {
			d.ServerPort = 53
		}
		dnsProber := prober.NewDNS(d.Name, d.Domain, d.RecordType, d.RPS, d.ServerIP, d.ServerPort, d.Timeout)

		klog.Infof("Starting DNS probe '%s' with domain '%s', RecordType: %s, RPS: %.2f, server: %s, port: %v, timeout: %s",
			d.Name, d.Domain, d.RecordType, d.RPS, d.ServerIP, d.ServerPort, d.Timeout)
		
		wg.Add(1)
		go func() {
			defer wg.Done()
			dnsProber.Start(ctx)
		}()
	}

	// Start K8S probes if enabled
	if config.Get().Targets.K8S.Enabled {
		klog.Info("K8S Prober is Enabled")
		k8sClient := prober.Getk8sClient()
		for _, sp := range config.Get().Targets.K8S.SimpleProbe {
			k8sSimpleProber := prober.NewSimpleProbe(k8sClient, sp.NameSpace, sp.RPS)
			klog.Infof("Starting K8S probe for namespace '%s', RPS: %.2f",
				sp.NameSpace, sp.RPS)
			
			wg.Add(1)
			go func() {
				defer wg.Done()
				k8sSimpleProber.Start(ctx)
			}()
		}
	} else {
		klog.Info("K8S Prober is Disabled")
	}

	// Start ICMP probes
	for _, i := range config.Get().Targets.ICMP {
		icmpProber := prober.NewICMP(i.Name, i.Host, i.RPS, i.TTL, i.Timeout)
		klog.Infof("Starting ICMP probe '%s' with host '%s', RPS: %.2f, timeout: %s, ttl: %v",
			i.Name, i.Host, i.RPS, i.Timeout, i.TTL)
		
		wg.Add(1)
		go func() {
			defer wg.Done()
			icmpProber.Start(ctx)
		}()
	}

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Start server in a goroutine
	server := &http.Server{
		Addr:    config.Get().Listen,
		Handler: mux,
	}

	go func() {
		klog.Infof("Starting HTTP server on %s", config.Get().Listen)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	klog.Info("Shutting down...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		klog.Errorf("HTTP server shutdown error: %v", err)
	}

	// Wait for all probes to finish
	wg.Wait()
	klog.Info("Shutdown complete")
}
