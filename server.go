package main

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	klog "k8s.io/klog/v2"
)

func setupServer(port int) error {
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	klog.Infof("Starting HTTP server on port %d", port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
