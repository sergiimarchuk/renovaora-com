# Stage 1: Build the Go binary
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy module files first for better caching
COPY go.mod ./
# If go.sum exists later, uncomment the next line
# COPY go.sum ./

# Copy the rest of the source code
COPY . .

# Build the application statically
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/web/main.go

# Stage 2: Final lightweight image
FROM alpine:3.20

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/main .

# Copy the UI directory (templates and assets) since Go renders them at runtime
COPY --from=builder /app/ui ./ui

# Expose the application port
EXPOSE 8080

# Run the binary
CMD ["./main"]

