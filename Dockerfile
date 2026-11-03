# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS server
WORKDIR /build/server
COPY server/go.mod server/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY server/ ./
# Ignore prebuilt workspace assets; only embed the frontend built above.
COPY --from=web /build/web/dist/ ./internal/webembed/dist/
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/pier ./cmd/server && \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/pier-crypt ./cmd/pier-crypt

# Official Docker CLI image includes the Compose and Buildx plugins. No daemon
# runs inside this container; host Docker access is an optional socket mount.
FROM docker:29-cli AS runtime
RUN apk add --no-cache bash ca-certificates curl gzip tar tzdata \
    && mkdir -p /app /data/pier/state /data/pier/apps \
    && chmod 0700 /data/pier/state
COPY --from=server /out/pier /out/pier-crypt /usr/local/bin/
WORKDIR /app
ENV PIER_LISTEN=0.0.0.0:8099 \
    PIER_DATA_DIR=/data/pier/state \
    PIER_PROJECTS_DIR=/data/pier/apps \
    PIER_CONTAINER=1
EXPOSE 8099
VOLUME ["/data/pier"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -q --spider http://127.0.0.1:8099/healthz || exit 1
STOPSIGNAL SIGTERM
ENTRYPOINT ["pier"]
CMD []
