# Build the web app.
FROM oven/bun:1 AS web
WORKDIR /src/web
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY web/ ./
RUN bun run build

# Build the API server.
FROM golang:1.26-alpine AS backend
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /out/fanti ./cmd/fanti

# Runtime: one image serving API + SPA.
FROM alpine:3.21
RUN adduser -D -u 10001 fanti \
    && mkdir -p /app/data \
    && chown -R fanti:fanti /app
WORKDIR /app
COPY --from=backend /out/fanti /app/fanti
COPY --from=web /src/web/dist /app/web
# Keep immutable bootstrap inputs outside /app/data, which is a persistent volume.
COPY --from=backend --chown=fanti:fanti /src/backend/data/downloads /app/datasets
COPY --chmod=755 --chown=fanti:fanti deploy/docker-entrypoint.sh /app/entrypoint.sh
USER fanti
ENV FANTI_STATIC_DIR=/app/web \
    FANTI_LISTEN_ADDR=:8080 \
    FANTI_BLOB_DIR=/app/data/blobs
EXPOSE 8080
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["serve"]
