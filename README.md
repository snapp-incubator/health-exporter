# Health Exporter

A prometheus exporter for continous health check of different endpoints.

In comparision to blackbox-exporter which probes endpoints on each prometheus scrape, this exporter continuously probes the endpoints with configured RPS, so prometheus can scrape on a longer interval, and calculate success rate, latency avg or percentiles, with a much higher resolution. Also this can be used to health-check an endpoint with a high RPS synthetic load.


## Build

### Docker

`docker build -t health-exporter .`


### Binary

```bash
git clone --depth 1 https://github.com/snapp-cab/health-exporter.git
cd health-exporter
go build
```

## Installation


### Docker


```bash
sudo docker run -p 8080:8080 -v ./config.yaml:/app/config.yaml:z docker.pkg.github.com/snapp/health-exporter/image:latest
```

### Helm chart

* Prerequisites
  * **Helm 3.0+** (Helm 2 is not supported)
  * **Kubernetes 1.10+** - This is the earliest version of Kubernetes tested.
    It is possible that this chart works with earlier versions but it is
    untested.


1. Add the SnappCab Helm Repository:

```bash
helm repo add snapp-cab https://snapp-cab.github.io/health-exporter/charts
helm repo update
```

2. Install with:

```bash
helm install health-exporter snapp-cab/health-exporter
```

### Binary releases

```bash
export VERSION=1.0.0
wget https://github.com/cafebazaar/health-exporter/releases/download/v${VERSION}/health-exporter-${VERSION}.linux-amd64.tar.gz
tar xvzf health-exporter-${VERSION}.linux-amd64.tar.gz health-exporter-${VERSION}.linux-amd64/health-exporter
```

## Usage

Run health-exporter

```bash
health-exporter -config <PathToConfig>
```

See [example configuration file](config.example.yaml) for sample config file.

Currently, health-exporter supports these probes:

Name    | Description
--------|------------
http    | For http(s) calls to different endpoints
dns     | For dns requests to different DNS servers
k8s     | For k8s-api health check


## Metrics


| Metric                                          | Notes
|-------------------------------------------------|------------------------------------
| health_http_requests_total                      | Total number of http requests
| health_http_duration_seconds_count              | Total number of http requests
| health_http_duration_seconds_sum                | Duration of http requests with response code
| health_http_duration_seconds_bucket             | Count of http requests per bucket (for calculating percentile)
| health_dns_requests_total                       | Total number of dns requests
| health_dns_duration_seconds_count               | Total number of dns requests
| health_dns_duration_seconds_sum                 | Duration of dns requests with response code
| health_dns_duration_seconds_bucket              | Count of dns requests per bucket (for calculating percentile)
| health_k8s_http_request_total                   | Total number of k8s api requests
| health_k8s_http_request_duration_seconds_count  | Total number of k8s api requests
| health_k8s_http_request_duration_seconds_sum    | Duration of k8s api requests with response code
| health_k8s_pod_count                            | Number of pods (used as a health-check for api-server)


## Security

### Reporting security vulnerabilities

If you find a security vulnerability or any security related issues, please DO NOT file a public issue, instead send your report privately to cloud@snapp.cab. Security reports are greatly appreciated and we will publicly thank you for it.

## License

Apache-2.0 License, see [LICENSE](LICENSE).
