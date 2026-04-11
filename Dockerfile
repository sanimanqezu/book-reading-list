# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Download dependencies first (cached layer unless go.mod/go.sum change).
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically linked binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/api

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.19

# ca-certificates is required for HTTPS calls to the Open Library API.
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the compiled binary from the builder stage.
COPY --from=builder /app/server .

# Copy migration files so the binary can find them at runtime.
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080

CMD ["./server"]
