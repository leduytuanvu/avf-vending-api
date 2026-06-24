# One-shot migrations: goose CLI on Alpine (matches prod migrate service).
# Base image digests: multi-arch index (library/golang, library/alpine on Docker Hub). Bump digests when changing GO_VERSION / Alpine tag.
# XNET_VERSION / XCRYPTO_VERSION pin patched golang.org/x deps so the goose binary clears Trivy HIGH/CRITICAL policy; bump when newer CVEs are flagged.
ARG GO_VERSION=1.25.11
ARG GOOSE_VERSION=v3.27.1
ARG PGX_VERSION=v5.9.2
ARG GRPC_VERSION=v1.80.0

FROM golang:1.25.11-alpine@sha256:8d95af53d0d58e1759ddb4028285d9b1239067e4fbf4f544618cad0f60fbc354 AS build
ARG GOOSE_VERSION
ARG PGX_VERSION
ARG GRPC_VERSION
ARG XNET_VERSION=v0.56.0
ARG XCRYPTO_VERSION=v0.53.0
WORKDIR /src
RUN apk add --no-cache git \
    && git clone --branch "${GOOSE_VERSION}" --depth 1 https://github.com/pressly/goose.git . \
    && go mod edit -require=github.com/jackc/pgx/v5@"${PGX_VERSION}" \
    && go mod edit -require=google.golang.org/grpc@"${GRPC_VERSION}" \
    && go mod edit -require=golang.org/x/net@"${XNET_VERSION}" \
    && go mod edit -require=golang.org/x/crypto@"${XCRYPTO_VERSION}" \
    && go mod tidy \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/goose ./cmd/goose

FROM alpine:3.24@sha256:a2d49ea686c2adfe3c992e47dc3b5e7fa6e6b5055609400dc2acaeb241c829f4
RUN apk add --no-cache ca-certificates \
    && apk upgrade --no-cache libcrypto3 libssl3 openssl
COPY --from=build /out/goose /usr/local/bin/goose
ENTRYPOINT ["/usr/local/bin/goose"]
