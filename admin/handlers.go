package main

import "net/http"

// Stub handlers — placeholders so the skeleton compiles.
// Real logic (rendering the login form, checking credentials, creating
// sessions, etc.) is implemented in the next step.

func handleLoginPage(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("login page not yet implemented"))
}

func handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("login submit not yet implemented"))
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("logout not yet implemented"))
}
