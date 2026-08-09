package main

import (
	"context"
	"net/http"
)

// contextKey avoids collisions with other packages' context keys.
type contextKey string

const userIDContextKey contextKey = "userID"

// AuthRequired wraps a handler so it only runs if the request has a valid,
// non-expired session cookie. Otherwise it redirects to /login.
func AuthRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		session, err := GetValidSession(r.Context(), cookie.Value)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if session == nil {
			http.Redirect(w, r, "/login?timeout=1", http.StatusFound)
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey, session.UserID)
		next(w, r.WithContext(ctx))
	}
}
