# ── Build stage ────────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o pr-reviewer ./cmd/server

# ── Run stage ──────────────────────────────────────────────────────────────────
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /build/pr-reviewer .

EXPOSE 8080
ENTRYPOINT ["/app/pr-reviewer"]
