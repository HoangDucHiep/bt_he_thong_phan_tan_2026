# ── Stage 1: Build ─────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Download dependencies first (layer-cache friendly)
COPY go.mod go.sum ./
RUN go mod download

# Build binary (CGO disabled — modernc.org/sqlite is pure Go)
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o user-service .

# ── Stage 2: Runtime ───────────────────────────────────────────────────────────
FROM alpine:3.19

WORKDIR /app

# ca-certificates is useful if the service ever makes HTTPS calls
RUN apk --no-cache add ca-certificates

COPY --from=builder /app/user-service ./

EXPOSE 8081

CMD ["./user-service"]
