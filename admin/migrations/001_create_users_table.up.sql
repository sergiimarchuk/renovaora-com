CREATE TABLE users (
    id            SERIAL PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

-- Speeds up login lookups; partial index skips soft-deleted rows
CREATE UNIQUE INDEX idx_users_username_active
    ON users (username)
    WHERE deleted_at IS NULL;
