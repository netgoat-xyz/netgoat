# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY main.go ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -buildid=" -o /out/dns .

FROM busybox:1.37.0-musl AS healthcheck

FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.title="NetGoat authoritative DNS service" \
      org.opencontainers.image.description="Authoritative DNS service backed by NetGoat stream-server" \
      org.opencontainers.image.source="https://github.com/netgoat-xyz/netgoat"
WORKDIR /app
COPY --from=healthcheck /bin/busybox /busybox
COPY --from=build --chown=nonroot:nonroot /out/dns /app/dns
USER nonroot:nonroot
ENV DNS_ADDR=:1053 \
    DNS_HTTP_ADDR=:8053 \
    DNS_POLL_SECONDS=5 \
    GOMEMLIMIT=128MiB
EXPOSE 1053/udp 1053/tcp 8053/tcp
HEALTHCHECK --interval=10s --timeout=3s --retries=8 --start-period=20s \
  CMD ["/busybox", "wget", "-qO-", "http://127.0.0.1:8053/readyz"]
ENTRYPOINT ["/app/dns"]
