# One-shot migrations: goose CLI on Alpine (matches prod migrate service).
# Base image digests: multi-arch index (library/golang, library/alpine on Docker Hub). Bump digests when changing GO_VERSION / Alpine tag.
ARG GO_VERSION=1.25.9
ARG GOOSE_VERSION=v3.27.0
ARG PGX_VERSION=v5.9.0
ARG GRPC_VERSION=v1.79.3

FROM golang:1.25.9-alpine@sha256:5caaf1cca9dc351e13deafbc3879fd4754801acba8653fa9540cea125d01a71f AS build
ARG GOOSE_VERSION
ARG PGX_VERSION
ARG GRPC_VERSION
WORKDIR /src
RUN apk add --no-cache git \
    && git clone --branch "${GOOSE_VERSION}" --depth 1 https://github.com/pressly/goose.git . \
    && go mod edit -require=github.com/jackc/pgx/v5@"${PGX_VERSION}" \
    && go mod edit -require=google.golang.org/grpc@"${GRPC_VERSION}" \
    && go mod tidy \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/goose ./cmd/goose

FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc
RUN apk add --no-cache ca-certificates
COPY --from=build /out/goose /usr/local/bin/goose
ENTRYPOINT ["/usr/local/bin/goose"]
