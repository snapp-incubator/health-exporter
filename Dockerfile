#build stage
FROM golang:1.17 AS builder
RUN mkdir -p /go/src/app
WORKDIR /go/src/app

COPY go.sum go.mod /go/src/app/
RUN go mod download

COPY . /go/src/app
RUN go build -ldflags="-w -s" -o health_exporter

#final stage
FROM gcr.io/distroless/base
WORKDIR /app
COPY --from=builder /go/src/app/health_exporter /app/health_exporter
CMD ["/app/health_exporter"]
LABEL Name=health_exporter
EXPOSE 9876
