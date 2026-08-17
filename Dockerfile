# syntax=docker/dockerfile:1.7

FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal/bot ./internal/bot
COPY internal/config ./internal/config
COPY internal/db ./internal/db
COPY internal/service ./internal/service
COPY ent ./ent

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 go build -buildvcs=false -ldflags="-s -w" -o /out/alt-bot ./cmd/bot

FROM alpine:3.24

RUN apk add --no-cache ca-certificates && \
    adduser -D -H appuser

USER appuser
WORKDIR /home/appuser

COPY --from=builder /out/alt-bot ./alt-bot
COPY internal/assets ./internal/assets

ENTRYPOINT ["./alt-bot"]
