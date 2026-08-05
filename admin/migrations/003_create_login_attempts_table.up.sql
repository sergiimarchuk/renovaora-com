CREATE TABLE login_attempts (
    id           SERIAL PRIMARY KEY,
    username     TEXT NOT NULL,
    ip_address   INET NOT NULL,
    success      BOOLEAN NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Used to quickly count recent failed attempts per username/IP for lockout logic
CREATE INDEX idx_login_attempts_username_time ON login_attempts (username, attempted_at);
CREATE INDEX idx_login_attempts_ip_time ON login_attempts (ip_address, attempted_at);
