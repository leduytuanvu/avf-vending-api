# Production image: all Go binaries in one small image (pick command in Compose).
# Build from repository root: docker build -f deployments/prod/Dockerfile .
# Base image digests: multi-arch index (library/golang, library/alpine on Docker Hub). Bump digests when changing GO_VERSION / Alpine tag.

ARG GO_VERSION=1.26.6
# Injected by CI (Build and Push) so /version git_sha and build_time match the built commit.
ARG GIT_SHA=
ARG BUILD_TIME=

FROM golang:1.27-bookworm@sha256:4cdeb0986daca9cfb8b0485f762778a149206025d80ae4212fde9a8dacedca55 AS build
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
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="${LDFLAGS}" -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="${LDFLAGS}" -o /out/bootstrap-admin ./cmd/bootstrap-admin

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
RUN apk add --no-cache ca-certificates tzdata curl \
    && apk upgrade --no-cache libcrypto3 libssl3 openssl \
    && addgroup -S avf && adduser -S avf -G avf

WORKDIR /app
COPY migrations /app/migrations
COPY --from=build /out/api /app/api
COPY --from=build /out/worker /app/worker
COPY --from=build /out/mqtt-ingest /app/mqtt-ingest
COPY --from=build /out/reconciler /app/reconciler
COPY --from=build /out/temporal-worker /app/temporal-worker
COPY --from=build /out/migrate /app/migrate
COPY --from=build /out/bootstrap-admin /app/bootstrap-admin

USER avf
EXPOSE 8080

# Default CMD overridden per service in docker-compose.prod.yml
CMD ["/app/api"]
