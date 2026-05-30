# Production image: all Go binaries in one small image (pick command in Compose).
# Build from repository root: docker build -f deployments/prod/Dockerfile .
# Base image digests: multi-arch index (library/golang, library/alpine on Docker Hub). Bump digests when changing GO_VERSION / Alpine tag.

ARG GO_VERSION=1.25.10
# Injected by CI (Build and Push) so /version git_sha and build_time match the built commit.
ARG GIT_SHA=
ARG BUILD_TIME=

FROM golang:1.25.10-bookworm@sha256:e3a54b77385b4f8a31c1db4d12429ffb3718ea76865731a787c497755d409547 AS build
WORKDIR /src

ARG GIT_SHA=
ARG BUILD_TIME=
ARG VERSION=dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN mkdir -p /out && \
    LDFLAGS="-s -w \
      -X github.com/avf/avf-vending-api/internal/version.Version=${VERSION} \
      -X github.com/avf/avf-vending-api/internal/version.Commit=${GIT_SHA} \
      -X github.com/avf/avf-vending-api/internal/version.BuildTime=${BUILD_TIME}" && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="${LDFLAGS}" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="${LDFLAGS}" -o /out/worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="${LDFLAGS}" -o /out/mqtt-ingest ./cmd/mqtt-ingest && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="${LDFLAGS}" -o /out/reconciler ./cmd/reconciler && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="${LDFLAGS}" -o /out/temporal-worker ./cmd/temporal-worker && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="${LDFLAGS}" -o /out/migrate ./cmd/migrate

FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc
RUN apk add --no-cache ca-certificates tzdata curl \
    && addgroup -S avf && adduser -S avf -G avf

WORKDIR /app
COPY migrations /app/migrations
COPY --from=build /out/api /app/api
COPY --from=build /out/worker /app/worker
COPY --from=build /out/mqtt-ingest /app/mqtt-ingest
COPY --from=build /out/reconciler /app/reconciler
COPY --from=build /out/temporal-worker /app/temporal-worker
COPY --from=build /out/migrate /app/migrate

USER avf
EXPOSE 8080

# Default CMD overridden per service in docker-compose.prod.yml
CMD ["/app/api"]
