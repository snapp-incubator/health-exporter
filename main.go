package main

import (
	"context"
	"flag"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
	"net/http"

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
	klog.Infof("Using config file: %s", configPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start all configured probers
	prober.StartAll(ctx, config.Get().Targets)

	// Setup HTTP metrics handler
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	// Launch HTTP server with signal handling
	startServer(config.Get().Listen, mux, cancel)
}
