#!/bin/sh

go test ./... && \
CGO_ENABLED=0 go build \
  -o service-wrapper \
  -ldflags 'extldflags="-static"' \
  ./cmd/service-wrapper && \
upx service-wrapper
