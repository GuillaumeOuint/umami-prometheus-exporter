# syntax=docker/dockerfile:1
FROM golang:1.25.1 AS builder

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /workspace/umami-exporter ./cmd/exporter

FROM alpine:3.18

RUN apk add --no-cache ca-certificates

COPY --from=builder /workspace/umami-exporter /usr/local/bin/umami-exporter

ENV UMAMI_REFRESH_INTERVAL=1m
EXPOSE 9465

ENTRYPOINT ["/usr/local/bin/umami-exporter"]