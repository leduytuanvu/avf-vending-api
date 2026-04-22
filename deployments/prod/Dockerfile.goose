# One-shot migrations: goose CLI on Alpine (matches prod migrate service).
ARG GO_VERSION=1.25.9

FROM golang:${GO_VERSION}-alpine AS build
RUN apk add --no-cache git \
    && go install github.com/pressly/goose/v3/cmd/goose@v3.27.0

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /go/bin/goose /usr/local/bin/goose
ENTRYPOINT ["/usr/local/bin/goose"]
