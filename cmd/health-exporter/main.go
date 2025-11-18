package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"

	klog "k8s.io/klog/v2"

	"gitlab.snapp.ir/snappcloud/health_exporter/internal/app"
	"gitlab.snapp.ir/snappcloud/health_exporter/internal/config"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path of config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		klog.Fatalf("Cannot read/parse config file: %v", err)
	}
	klog.Infof("Using config file %q", *configPath)

	application, err := app.New(cfg)
	if err != nil {
		klog.Fatalf("failed to initialize app: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := application.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		klog.Fatalf("application stopped: %v", err)
	}
}
