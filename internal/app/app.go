package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
	klog "k8s.io/klog/v2"

	"gitlab.snapp.ir/snappcloud/health_exporter/internal/config"
	"gitlab.snapp.ir/snappcloud/health_exporter/internal/metrics"
	"gitlab.snapp.ir/snappcloud/health_exporter/internal/probe"
	dnsprobe "gitlab.snapp.ir/snappcloud/health_exporter/internal/probe/dns"
	httpprobe "gitlab.snapp.ir/snappcloud/health_exporter/internal/probe/http"
	icmpprobe "gitlab.snapp.ir/snappcloud/health_exporter/internal/probe/icmp"
	k8sprobe "gitlab.snapp.ir/snappcloud/health_exporter/internal/probe/k8s"
	"gitlab.snapp.ir/snappcloud/health_exporter/internal/server"
)

type App struct {
	cfg     config.Config
	server  *server.Server
	probes  []probe.Runner
	reg     prometheus.Registerer
	metrics struct {
		http *metrics.HTTP
		dns  *metrics.DNS
		icmp *metrics.ICMP
		k8s  *metrics.K8S
	}
}

func New(cfg config.Config) (*App, error) {
	app := &App{
		cfg: cfg,
		reg: prometheus.DefaultRegisterer,
	}

	app.metrics.http = metrics.NewHTTP(app.reg)
	app.metrics.dns = metrics.NewDNS(app.reg)
	app.metrics.icmp = metrics.NewICMP(app.reg)
	app.metrics.k8s = metrics.NewK8S(app.reg)

	if err := app.buildProbes(); err != nil {
		return nil, err
	}

	app.server = server.New(cfg.Listen, newMux())

	return app, nil
}

func (a *App) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	for _, runner := range a.probes {
		runner := runner
		g.Go(func() error {
			return runner.Run(ctx)
		})
	}

	g.Go(func() error {
		return a.server.Run(ctx)
	})

	return g.Wait()
}

func (a *App) buildProbes() error {
	for _, target := range a.cfg.Targets.HTTP {
		klog.Infof("Configuring HTTP probe %q url=%s rps=%.2f timeout=%s", target.Name, target.URL, target.RPS, target.Timeout)
		a.probes = append(a.probes, httpprobe.New(target, a.metrics.http))
	}

	for _, target := range a.cfg.Targets.DNS {
		klog.Infof("Configuring DNS probe %q domain=%s rps=%.2f server=%s:%d", target.Name, target.Domain, target.RPS, target.ServerIP, target.ServerPort)
		a.probes = append(a.probes, dnsprobe.New(target, a.metrics.dns))
	}

	for _, target := range a.cfg.Targets.ICMP {
		klog.Infof("Configuring ICMP probe %q host=%s rps=%.2f ttl=%d timeout=%s", target.Name, target.Host, target.RPS, target.TTL, target.Timeout)
		a.probes = append(a.probes, icmpprobe.New(target, a.metrics.icmp))
	}

	if a.cfg.Targets.K8S.Enabled {
		if err := a.setupK8SProbes(); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) setupK8SProbes() error {
	klog.Infof("Kubernetes probing enabled")
	metrics.RegisterClientGoMetrics(a.reg)

	client, err := k8sprobe.NewClient()
	if err != nil {
		return fmt.Errorf("k8s client: %w", err)
	}

	for _, target := range a.cfg.Targets.K8S.SimpleProbe {
		klog.Infof("Configuring K8S simple probe namespace=%s rps=%.2f", target.NameSpace, target.RPS)
		a.probes = append(a.probes, k8sprobe.NewSimpleProbe(client, target, a.metrics.k8s))
	}
	return nil
}

func newMux() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}
