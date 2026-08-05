CREATE TABLE sessions (
    id             TEXT PRIMARY KEY,           -- secure random token, generated in Go, not by the DB
    user_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_activity  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at     TIMESTAMPTZ NOT NULL        -- hard cap, independent of activity
);

-- Fast lookup of all sessions for a given user (e.g. "log out everywhere")
CREATE INDEX idx_sessions_user_id ON sessions (user_id);

-- Helps a periodic cleanup job find expired rows quickly
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);
