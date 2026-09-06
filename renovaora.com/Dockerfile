# Stage 1: Build the Go binary
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/web/main.go

# Stage 2: Final lightweight image
FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=builder /app/ui ./ui
COPY --from=builder /app/internal/i18n ./internal/i18n
EXPOSE 8080
CMD ["./main"]
