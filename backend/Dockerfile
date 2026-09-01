FROM golang:1.27.0-alpine3.23@sha256:3747dcba41c8b0db3211fda4db61638b980e17ac5bb3c94460a975a9cfe19395 AS build

WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./
COPY backend/cmd ./cmd
COPY backend/internal ./internal

RUN go build -trimpath -ldflags="-s -w" -o /out/ause-api ./cmd/api

FROM golang:1.27.0-alpine3.23@sha256:3747dcba41c8b0db3211fda4db61638b980e17ac5bb3c94460a975a9cfe19395

RUN addgroup -S ause && \
    adduser -S -G ause ause && \
    mkdir -p /var/lib/ause-discovery && \
    chown ause:ause /var/lib/ause-discovery

COPY --from=build /out/ause-api /usr/local/bin/ause-api

USER ause

ENTRYPOINT ["/usr/local/bin/ause-api"]
