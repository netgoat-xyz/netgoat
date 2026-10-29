# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -buildid=" -o /out/netgoat . \
    && mkdir -p /out/database

FROM busybox:1.37.0-musl AS healthcheck

FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.description="Minimal hardened NetGoat edge proxy runtime" \
      org.opencontainers.image.base.name="gcr.io/distroless/static-debian12:nonroot" \
      org.opencontainers.image.source="https://github.com/netgoat-xyz/netgoat"
WORKDIR /app
COPY --from=healthcheck /bin/busybox /busybox
COPY --from=build --chown=nonroot:nonroot /out/netgoat /app/netgoat
COPY --from=build --chown=nonroot:nonroot /out/database /app/database
COPY --chown=nonroot:nonroot public ./public
COPY --chown=nonroot:nonroot config.yml ./config.yml
USER nonroot:nonroot
EXPOSE 8080
ENV NO_COLOR=1 \
    GOMEMLIMIT=192MiB
# TCP liveness probe: the agent answers 404/405 for unconfigured hosts and
# paths, so an HTTP status check would report a healthy server as failing.
# The probe port follows NETGOAT_HEALTHCHECK_PORT because the mounted
# config.yml can move the listener (ssl.enabled, or a custom listen address).
HEALTHCHECK --interval=15s --timeout=5s --retries=10 --start-period=25s \
  CMD ["/busybox", "sh", "-c", "/busybox nc -z 127.0.0.1 \"${NETGOAT_HEALTHCHECK_PORT:-8080}\""]
ENTRYPOINT ["/app/netgoat"]
