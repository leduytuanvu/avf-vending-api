# One-shot migrations: goose CLI on Alpine (matches prod migrate service).
FROM golang:1.26-alpine AS build
RUN apk add --no-cache git \
    && go install github.com/pressly/goose/v3/cmd/goose@v3.24.1

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /go/bin/goose /usr/local/bin/goose
ENTRYPOINT ["/usr/local/bin/goose"]
