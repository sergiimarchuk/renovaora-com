#!/bin/sh
set -e

PGDATA_DIR="/var/lib/postgresql/data"
PG_LOG="/var/lib/postgresql/data/log.txt"

# --- 1. Initialize the Postgres data directory on first run only ---
if [ ! -s "$PGDATA_DIR/PG_VERSION" ]; then
    echo "[start.sh] No existing DB found — initializing Postgres data directory..."

    if [ -z "$POSTGRES_PASSWORD" ]; then
        echo "[start.sh] ERROR: POSTGRES_PASSWORD environment variable is required." >&2
        exit 1
    fi

    # Write the password to a temp file so initdb can read it without
    # the password appearing in process listings (ps aux)
    echo "$POSTGRES_PASSWORD" > /tmp/pg_pwfile
    su-exec postgres initdb -D "$PGDATA_DIR" \
        --auth=scram-sha-256 \
        --username="${POSTGRES_USER:-appuser}" \
        --pwfile=/tmp/pg_pwfile
    rm -f /tmp/pg_pwfile

    # Only accept connections from localhost inside this container —
    # nothing outside the container can reach this Postgres instance directly
    echo "listen_addresses = 'localhost'" >> "$PGDATA_DIR/postgresql.conf"
fi

# --- 2. Start Postgres ---
echo "[start.sh] Starting Postgres..."
su-exec postgres pg_ctl -D "$PGDATA_DIR" -l "$PG_LOG" -w start

# --- 3. Run schema migrations ---
echo "[start.sh] Running migrations..."
/app/migrate -path /app/migrations -database "$DATABASE_URL" up

# --- 4. Drop privileges and start the Go app ---
echo "[start.sh] Starting public-app as non-root user (appnutzer)..."
exec su-exec appnutzer /app/public-app
