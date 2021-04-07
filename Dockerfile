#build stage
FROM golang:1.16.3-buster AS builder
RUN mkdir -p /go/src/app
WORKDIR /go/src/app

COPY go.sum go.mod /go/src/app/
RUN go env -w GOPROXY="https://repo.snapp.tech/repository/goproxy/"
RUN go mod download

COPY . /go/src/app
RUN go build -ldflags="-w -s" -o health_exporter

#final stage
FROM debian:buster-slim

ENV TZ=UTC \
    PATH="/app:${PATH}"

RUN DEBIAN_FRONTEND=noninteractive apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /app

COPY --from=builder /go/src/app/health_exporter /app/health_exporter
ENTRYPOINT /app/health_exporter
LABEL Name=health_exporter Version=1.0.0
EXPOSE 9876
