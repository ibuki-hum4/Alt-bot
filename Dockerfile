FROM golang:1.26-alpine AS builder

WORKDIR /app


COPY go.mod go.sum ./
RUN go mod download


COPY . .

RUN CGO_ENABLED=0 go build -buildvcs=false -ldflags="-s -w" -o /out/alt-bot ./cmd/bot


FROM alpine:3.23

RUN apk add --no-cache ca-certificates && \
    adduser -D -H appuser

USER appuser
WORKDIR /home/appuser

COPY --from=builder /out/alt-bot ./alt-bot

ENTRYPOINT ["./alt-bot"]
