# ── Stage 1: Build ─────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /app

# api-gateway uses only stdlib — no external dependencies, no go.sum needed
COPY go.mod ./

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o api-gateway .

# ── Stage 2: Runtime ───────────────────────────────────────────────────────────
FROM alpine:3.19

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/api-gateway ./

EXPOSE 8080

CMD ["./api-gateway"]
