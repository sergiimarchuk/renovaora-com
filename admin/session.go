package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"
)

// sessionDuration is the hard cap on how long a session can live,
// regardless of activity.
const sessionDuration = 60 * time.Minute

// inactivityTimeout logs the user out if this much time passes with no
// requests, even before sessionDuration is reached.
const inactivityTimeout = 30 * time.Minute

// Session mirrors a row in the sessions table.
type Session struct {
	ID           string
	UserID       int
	CreatedAt    time.Time
	LastActivity time.Time
	ExpiresAt    time.Time
}

// generateSessionToken returns a cryptographically random, URL-safe token.
// 32 bytes = 256 bits of entropy — not practically guessable.
func generateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// CreateSession generates a new token and inserts a session row for the
// given user.
func CreateSession(ctx context.Context, userID int) (*Session, error) {
	token, err := generateSessionToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	expiresAt := now.Add(sessionDuration)

	_, err = db.Exec(ctx,
		`INSERT INTO sessions (id, user_id, created_at, last_activity, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		token, userID, now, now, expiresAt,
	)
	if err != nil {
		return nil, err
	}

	return &Session{
		ID:           token,
		UserID:       userID,
		CreatedAt:    now,
		LastActivity: now,
		ExpiresAt:    expiresAt,
	}, nil
}

// GetValidSession looks up a session by token. Returns nil (no error) if
// the session doesn't exist, has hit its hard expiry, or has been idle
// past the inactivity timeout — in all those cases the caller should treat
// the user as logged out.
func GetValidSession(ctx context.Context, token string) (*Session, error) {
	s := &Session{}
	err := db.QueryRow(ctx,
		`SELECT id, user_id, created_at, last_activity, expires_at
		 FROM sessions WHERE id = $1`,
		token,
	).Scan(&s.ID, &s.UserID, &s.CreatedAt, &s.LastActivity, &s.ExpiresAt)
	if err != nil {
		return nil, nil // not found -> treat as no session, not a hard error
	}

	now := time.Now()

	if now.After(s.ExpiresAt) {
		_ = DeleteSession(ctx, token)
		return nil, nil
	}

	if now.Sub(s.LastActivity) > inactivityTimeout {
		_ = DeleteSession(ctx, token)
		return nil, nil
	}

	// Sliding activity: touch last_activity so the inactivity clock resets
	// on legitimate use.
	_, err = db.Exec(ctx,
		`UPDATE sessions SET last_activity = $1 WHERE id = $2`,
		now, token,
	)
	if err != nil {
		return nil, err
	}
	s.LastActivity = now

	return s, nil
}

// DeleteSession removes a session row — used for logout and for cleaning
// up expired/timed-out sessions.
func DeleteSession(ctx context.Context, token string) error {
	_, err := db.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, token)
	return err
}
