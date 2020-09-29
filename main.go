package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

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
		log.Fatalf("Cannot read/parse config file: %v", err)
	}

	log.Printf("Using config file '%s'\n", configPath)

	ctx, cancel := context.WithCancel(context.Background())

	for _, ht := range config.Get().Targets.HTTP {
		httpProber := prober.NewHttp(ht.Name, ht.URL, ht.RPS, ht.Timeout, ht.TLSSkipVerify)

		log.Printf("Probing HTTP target '%s' with url '%s', RPS: %.2f, timeout: %s, TLS_skip_verify: %v ...\n",
			ht.Name, ht.URL, ht.RPS, ht.Timeout, ht.TLSSkipVerify)
		go httpProber.Start(ctx)
	}

	for _, d := range config.Get().Targets.DNS {
		dnsProber := prober.NewDNS(d.Name, d.Domain, d.RecordType, d.RPS, d.Timeout)

		log.Printf("Probing DNS target '%s' with domain '%s', RecordType: %s, RPS: %.2f, timeout: %s ...\n",
			d.Name, d.Domain, d.RecordType, d.RPS, d.Timeout)
		go dnsProber.Start(ctx)
	}
	if config.Get().Targets.K8S.Enabled {
		log.Printf("K8S Prober is Enabled")
		k8s_client := prober.Getk8sClient()
		for _, sp := range config.Get().Targets.K8S.SimpleProbe {
			k8s_simpeProber := prober.NewSimpleProbe(k8s_client, sp.NameSpace, sp.RPS)
			log.Printf("Probing K8S target namespace '%s', RPS: 1.0 ...\n",
				sp.NameSpace)
			go k8s_simpeProber.Start(ctx)
		}
	} else {log.Printf("K8S Prober is Disabled")}
	
	

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	startServer(config.Get().Listen, mux, cancel)
}
