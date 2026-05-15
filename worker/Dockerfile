# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /worker .

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.21

RUN addgroup -S nonroot && adduser -S nonroot -G nonroot

COPY --from=builder /worker /worker

USER nonroot:nonroot

ENTRYPOINT ["/worker"]
