# One-shot migrations: goose CLI on Alpine (matches prod migrate service).
# Base image digests: multi-arch index (library/golang, library/alpine on Docker Hub). Bump digests when changing GO_VERSION / Alpine tag.
ARG GO_VERSION=1.25.10
ARG GOOSE_VERSION=v3.27.1
ARG PGX_VERSION=v5.9.2
ARG GRPC_VERSION=v1.80.0

FROM golang:1.26.3-alpine@sha256:91eda9776261207ea25fd06b5b7fed8d397dd2c0a283e77f2ab6e91bfa71079d AS build
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

FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11
RUN apk add --no-cache ca-certificates
COPY --from=build /out/goose /usr/local/bin/goose
ENTRYPOINT ["/usr/local/bin/goose"]
