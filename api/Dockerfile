# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /api .

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.21

RUN addgroup -S nonroot && adduser -S nonroot -G nonroot

COPY --from=builder /api /api

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/api"]
