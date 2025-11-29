# Health Exporter

Health Exporter continuously probes configured HTTP, DNS, ICMP, and Kubernetes targets across SnappCloud’s private regions and surfaces Prometheus metrics that describe endpoint reachability, pod health, and end-to-end network latency.  
Compared to scrape-on-demand approaches (e.g. blackbox-exporter) this dedicated process keeps issuing requests at configurable RPS so that aggregations—error-rates, latency percentiles, and private-cloud service availability—are always fresh when Prometheus scrapes `/metrics`.

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

All exported series map directly to probes that exercise private-cloud applications (public/private routers, inter-DC services, API servers, etc.) and the network planes that connect regions. They power alerting for both service health (HTTP success/error classification) and network latency (DNS lookup, TCP connect, ICMP).

| Metric                                          | Private cloud signal
|-------------------------------------------------|------------------------------------
| `health_http_requests_total`                    | Classified result per HTTP probe (private/public/edge routers, health-be endpoints)
| `health_http_duration_seconds_*`                | Latency histograms/counters for HTTP probes showing service responsiveness
| `health_http_dns_lookup_time_seconds`           | DNS lookup durations for HTTP probes, highlighting internal resolver slowness
| `health_dns_requests_total`                     | DNS probe result counters for cluster and inter-region domains
| `health_dns_duration_seconds_*`                 | DNS probe latency histograms for each resolver/IP
| `health_icmp_requests_total`                    | ICMP probe counters for network hops between regions/edges
| `health_icmp_duration_seconds_*`                | ICMP probe latency histograms capturing raw RTT between regions
| `health_k8s_http_request_total`                 | Kubernetes client-go HTTP metrics for API servers inside each private cloud
| `health_k8s_http_request_duration_seconds`      | Kubernetes API latency summaries
| `health_k8s_pod_count`                          | Gauge of pods per watched namespace, proving workloads are scheduled

## Deployment

- **Docker**: `docker run -p 9876:9876 -v $PWD/config.yaml:/app/config.yaml health-exporter`
- **Helm**: charts are available under `charts/health-exporter`

## Security

If you find a security vulnerability or any security related issues, please **do not** open a public issue.  
Send an email to cloud@snapp.cab and we will work with you; responsible disclosures are appreciated and credited.

## License

Apache-2.0 License, see [LICENSE](LICENSE).
