package main

import (
	"context"
	"fmt"
)

// User mirrors a row in the users table.
type User struct {
	ID           int
	Username     string
	PasswordHash string
}

// CreateUser hashes the given password and inserts a new user row.
// Not exposed via any public HTTP route — intended for a one-time CLI
// seed step, since this system has no public registration.
func CreateUser(ctx context.Context, username, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx,
		`INSERT INTO users (username, password_hash) VALUES ($1, $2)`,
		username, hash,
	)
	return err
}

// UpdateUserPassword hashes the given password and updates it for an
// existing active user. Returns an error if no matching user exists.
func UpdateUserPassword(ctx context.Context, username, newPassword string) error {
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	tag, err := db.Exec(ctx,
		`UPDATE users SET password_hash = $1
		 WHERE username = $2 AND deleted_at IS NULL`,
		hash, username,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("no active user found with username %q", username)
	}
	return nil
}

// GetUserByUsername looks up an active (non-deleted) user by username.
// Returns nil, nil if no matching user exists — callers should treat that
// the same as "invalid credentials", not reveal whether the username exists.
func GetUserByUsername(ctx context.Context, username string) (*User, error) {
	u := &User{}
	err := db.QueryRow(ctx,
		`SELECT id, username, password_hash FROM users
		 WHERE username = $1 AND deleted_at IS NULL`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash)
	if err != nil {
		return nil, nil
	}
	return u, nil
}
