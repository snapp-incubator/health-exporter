# Health Exporter

The Health Exporter continuously probes configured HTTP, DNS, ICMP, and Kubernetes targets and exposes Prometheus metrics that power the existing SLO/alerting rules.  
Compared to scrape-on-demand approaches (e.g. blackbox-exporter) this dedicated process keeps issuing requests at configurable RPS so that aggregations such as error rates or latency percentiles are always fresh when Prometheus scrapes `/metrics`.

## Requirements

- Go **1.25.4** (see `go.mod`) or a matching container image
- Access to the target network endpoints you want to probe

## Building

### Local binary

```bash
git clone https://github.com/snapp-cab/health-exporter.git
cd health-exporter
go build -o bin/health-exporter ./cmd/health-exporter
```

### Docker image

```bash
docker build -t health-exporter .
```

## Running

```bash
./bin/health-exporter -config config.yaml
```

See [config.example.yaml](config.example.yaml) for the configuration format. Each HTTP/DNS/ICMP probe declares a `name`, `url`/`domain`/`host`, requested `rps`, and timeout; optional fields let you toggle TLS verification, h2c, host headers, or DNS servers. Kubernetes probing is enabled via the `targets.k8s.enabled` flag and runs in-cluster using the service account.

## Metrics

The exporter continues to expose the following metric families — labels and names remain unchanged to keep the existing Alertmanager/Prometheus rules working:

| Metric                                          | Notes
|-------------------------------------------------|------------------------------------
| `health_http_requests_total`                    | Classified result per HTTP probe
| `health_http_duration_seconds_*`                | Latency histograms/counters for HTTP probes
| `health_http_dns_lookup_time_seconds`           | DNS lookup durations for HTTP probes
| `health_dns_requests_total`                     | DNS probe result counters
| `health_dns_duration_seconds_*`                 | DNS probe latency histograms
| `health_icmp_requests_total`                    | ICMP probe counters
| `health_icmp_duration_seconds_*`                | ICMP probe latency histograms
| `health_k8s_http_request_total`                 | Kubernetes client-go HTTP metrics
| `health_k8s_http_request_duration_seconds`      | Kubernetes client-go latency summaries
| `health_k8s_pod_count`                          | Gauge of pods per watched namespace

These are the exact series consumed by the provided `PrometheusRule`, so alert thresholds and dashboards do not need to change.

## Deployment

- **Docker**: `docker run -p 9876:9876 -v $PWD/config.yaml:/app/config.yaml health-exporter`
- **Helm**: charts are available under `charts/health-exporter`

## Security

If you find a security vulnerability or any security related issues, please **do not** open a public issue.  
Send an email to cloud@snapp.cab and we will work with you; responsible disclosures are appreciated and credited.

## License

Apache-2.0 License, see [LICENSE](LICENSE).
