FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY ent ./ent
COPY internal ./internal
RUN CGO_ENABLED=0 go build -buildvcs=false -o /out/alt-bot ./cmd/bot

FROM alpine:3.21

RUN adduser -D -H appuser
USER appuser
WORKDIR /home/appuser

COPY --from=builder /out/alt-bot ./alt-bot

ENTRYPOINT ["./alt-bot"]
