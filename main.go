package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/snapp-incubator/health-exporter/config"
	"github.com/snapp-incubator/health-exporter/prober"
	klog "k8s.io/klog/v2"
)

var (
	configFile = flag.String("config", "config.yaml", "Path to config file")
	port       = flag.Int("port", 9090, "Port to listen on")
)

func main() {
	flag.Parse()

	cfg, err := config.ReadConfig(*configFile)
	if err != nil {
		klog.Fatalf("Failed to read config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start HTTP server
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil); err != nil {
			klog.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// Start probes
	for _, probe := range cfg.Probes {
		switch probe.Type {
		case "http":
			p := prober.NewHttp(
				probe.Name,
				probe.URL,
				probe.RPS,
				time.Duration(probe.Timeout)*time.Second,
				probe.TLSSkipVerify,
				probe.DisableKeepAlive,
				probe.H2CEnabled,
				probe.Host,
			)
			go p.Start(ctx)
		case "dns":
			p := prober.NewDNS(
				probe.Name,
				probe.Host,
				probe.RPS,
				time.Duration(probe.Timeout)*time.Second,
				probe.Server,
			)
			go p.Start(ctx)
		case "icmp":
			p := prober.NewICMP(
				probe.Name,
				probe.Host,
				probe.RPS,
				time.Duration(probe.Timeout)*time.Second,
			)
			go p.Start(ctx)
		case "k8s":
			p := prober.NewK8s(
				probe.Name,
				probe.RPS,
				time.Duration(probe.Timeout)*time.Second,
				probe.K8sConfig,
			)
			go p.Start(ctx)
		default:
			klog.Warningf("Unknown probe type: %s", probe.Type)
		}
	}

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	klog.Info("Shutting down...")
}
