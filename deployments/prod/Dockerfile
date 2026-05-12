# Production image: all Go binaries in one small image (pick command in Compose).
# Build from repository root: docker build -f deployments/prod/Dockerfile .
# Base image digests: multi-arch index (library/golang, library/alpine on Docker Hub). Bump digests when changing GO_VERSION / Alpine tag.

ARG GO_VERSION=1.25.10

FROM golang:1.25.10-bookworm@sha256:e3a54b77385b4f8a31c1db4d12429ffb3718ea76865731a787c497755d409547 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN mkdir -p /out && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mqtt-ingest ./cmd/mqtt-ingest && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/reconciler ./cmd/reconciler && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/temporal-worker ./cmd/temporal-worker

FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11
RUN apk add --no-cache ca-certificates tzdata curl \
    && addgroup -S avf && adduser -S avf -G avf

WORKDIR /app
COPY --from=build /out/api /app/api
COPY --from=build /out/worker /app/worker
COPY --from=build /out/mqtt-ingest /app/mqtt-ingest
COPY --from=build /out/reconciler /app/reconciler
COPY --from=build /out/temporal-worker /app/temporal-worker

USER avf
EXPOSE 8080

# Default CMD overridden per service in docker-compose.prod.yml
CMD ["/app/api"]
