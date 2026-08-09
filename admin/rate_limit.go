package main

import (
	"context"
	"time"
)

// maxFailedAttempts is how many failed logins (for a given username OR IP)
// are allowed within lockoutWindow before further attempts are blocked.
const maxFailedAttempts = 5
const lockoutWindow = 15 * time.Minute

// RecordLoginAttempt logs every login attempt, success or failure. This
// both drives the rate-limit check below and doubles as a basic audit
// trail of login activity.
func RecordLoginAttempt(ctx context.Context, username, ipAddress string, success bool) error {
	_, err := db.Exec(ctx,
		`INSERT INTO login_attempts (username, ip_address, success, attempted_at)
		 VALUES ($1, $2, $3, $4)`,
		username, ipAddress, success, time.Now(),
	)
	return err
}

// IsLockedOut returns true if either the username or the IP address has
// hit the failed-attempt limit within the lockout window. Checking both
// independently means a distributed attempt against one username from many
// IPs is still caught (by username), and a single IP guessing many
// usernames is still caught (by IP).
func IsLockedOut(ctx context.Context, username, ipAddress string) (bool, error) {
	since := time.Now().Add(-lockoutWindow)

	var usernameFailures int
	err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM login_attempts
		 WHERE username = $1 AND success = false AND attempted_at > $2`,
		username, since,
	).Scan(&usernameFailures)
	if err != nil {
		return false, err
	}
	if usernameFailures >= maxFailedAttempts {
		return true, nil
	}

	var ipFailures int
	err = db.QueryRow(ctx,
		`SELECT COUNT(*) FROM login_attempts
		 WHERE ip_address = $1 AND success = false AND attempted_at > $2`,
		ipAddress, since,
	).Scan(&ipFailures)
	if err != nil {
		return false, err
	}
	if ipFailures >= maxFailedAttempts {
		return true, nil
	}

	return false, nil
}
