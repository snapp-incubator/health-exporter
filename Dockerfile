# build stage
FROM golang:1.25.4 AS builder
WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /workspace/health_exporter ./cmd/health-exporter

# final stage
FROM gcr.io/distroless/static
WORKDIR /app
COPY --from=builder /workspace/health_exporter /app/health_exporter
CMD ["/app/health_exporter"]
LABEL Name=health_exporter
EXPOSE 9876
