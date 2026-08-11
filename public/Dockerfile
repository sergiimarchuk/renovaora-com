# --- Build stage ---
FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static binary, no CGO, stripped symbols
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /public-app .

# Build the migrate CLI (used at container startup to apply schema migrations).
# Uses `go install` with a pinned version — builds in its own isolated
# module context, does NOT need to be listed in our project's go.mod/go.sum.
RUN CGO_ENABLED=0 GOOS=linux go install -ldflags="-s -w" -tags 'postgres' \
    github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.1 && \
    mv /root/go/bin/migrate /migrate

# --- Runtime stage ---
FROM alpine:3.20

# postgresql16 = the DB engine itself (bundled in this container per architecture decision)
# su-exec = lightweight way to drop from root to the postgres user for DB commands
# ca-certificates = needed for any outbound TLS
RUN apk add --no-cache postgresql16 su-exec ca-certificates && \
    mkdir -p /var/lib/postgresql/data && \
    mkdir -p /run/postgresql && \
    chown -R postgres:postgres /var/lib/postgresql /run/postgresql && \
    adduser -D -u 10001 appnutzer

WORKDIR /app

COPY --from=builder /public-app /app/public-app
COPY --from=builder /migrate /app/migrate
COPY templates ./templates
COPY migrations ./migrations
COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh

EXPOSE 8111

ENTRYPOINT ["/app/start.sh"]
