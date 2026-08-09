package main

import (
	"net"
	"net/http"
)

const sessionCookieName = "session_token"

// dummyHash is compared against when a username doesn't exist, so login
// takes roughly the same amount of time either way — avoids leaking
// "this username exists" via response timing.
const dummyHash = "$2a$12$C6UzMDM.H6dfI/f/IKcEeO/edEHDcv1E9F91U4M0OZDT6H7T9G0Xa"

func handleLoginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Admin Login</title></head>
<body>
	<h1>Admin Login</h1>
	<form method="POST" action="/login">
		<label>Username: <input type="text" name="username" required></label><br>
		<label>Password: <input type="password" name="password" required></label><br>
		<button type="submit">Log in</button>
	</form>
</body>
</html>`))
}

func handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")

	ip := clientIP(r)

	// Check lockout BEFORE touching the DB for the user lookup, so a
	// locked-out attacker can't keep hammering the password-check path.
	lockedOut, err := IsLockedOut(ctx, username, ip)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if lockedOut {
		http.Error(w, "too many failed attempts, try again later", http.StatusTooManyRequests)
		return
	}

	user, err := GetUserByUsername(ctx, username)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var passwordOK bool
	if user != nil {
		passwordOK = CheckPassword(password, user.PasswordHash)
	} else {
		// No such user — still run a bcrypt comparison against a dummy
		// hash so the response time doesn't reveal whether the username
		// exists.
		CheckPassword(password, dummyHash)
		passwordOK = false
	}

	// Record every attempt, success or failure — feeds both the rate
	// limiter and the audit trail.
	_ = RecordLoginAttempt(ctx, username, ip, passwordOK)

	if !passwordOK {
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}

	session, err := CreateSession(ctx, user.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.ID,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true, // not accessible via JavaScript — mitigates XSS token theft
		Secure:   true, // only sent over HTTPS (Caddy terminates TLS in front of this app)
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		_ = DeleteSession(ctx, cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1, // instructs the browser to delete the cookie immediately
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(w, r, "/login", http.StatusFound)
}

// clientIP extracts the request's source IP, stripping the port.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// handleDashboard is a placeholder for the authenticated admin landing
// page. Real content-management UI (pages/articles/media CRUD) is not yet
// built — this just confirms the auth gate works end-to-end.
func handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Admin Dashboard</title></head>
<body>
	<h1>Logged in</h1>
	<p>Content management UI not yet implemented.</p>
	<form method="POST" action="/logout">
		<button type="submit">Log out</button>
	</form>
</body>
</html>`))
}
