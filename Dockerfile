#
# 1. Build Container
#
FROM golang:1.11.4 AS build

ENV GO111MODULE=on \
    GOOS=linux \
    GOARCH=amd64

# We are not using Alpine for build stage as it does not include race-detection libraries
# which is required when running tests.
RUN apt-get update && \
    apt-get install -y \
      git \
      ca-certificates \
    && \
    mkdir -p /src

# First add modules list to better utilize caching
COPY go.sum go.mod /src/
WORKDIR /src
RUN go mod download
COPY . /src
RUN go build -o /app/main

#
# 2. Runtime Container
#
FROM alpine:3.8

ENV TZ=UTC \
    PATH="/app:${PATH}"

RUN apk add --update --no-cache \
      tzdata \
      ca-certificates \
      bash \
    && \
    cp --remove-destination /usr/share/zoneinfo/${TZ} /etc/localtime && \
    echo "${TZ}" > /etc/timezone && \
    mkdir -p /var/log && \
    chgrp -R 0 /var/log && \
    chmod -R g=u /var/log

WORKDIR /app

EXPOSE 8080

COPY --from=build /app /app/

CMD ["/app/main"]
