package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/term"
)

var db *pgxpool.Pool

func main() {
	ctx := context.Background()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	var err error
	db, err = pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}
	log.Println("connected to auth database")

	// CLI modes
	if len(os.Args) > 1 && os.Args[1] == "--create-user" {
		runCreateUser(ctx)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--reset-password" {
		runResetPassword(ctx)
		return
	}

	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /healthz", handleHealthz)

	// Auth routes
	mux.HandleFunc("GET /login", handleLoginPage)
	mux.HandleFunc("POST /login", handleLoginSubmit)
	mux.HandleFunc("POST /logout", handleLogout)

	// Dashboard
	mux.HandleFunc("GET /", AuthRequired(handleDashboard))
	mux.HandleFunc("GET /admin", AuthRequired(handleDashboard))

	// Pages
	mux.HandleFunc("GET /admin/pages", AuthRequired(handleAdminPages))
	mux.HandleFunc("GET /admin/pages/new", AuthRequired(handlePageNewForm))
	mux.HandleFunc("POST /admin/pages", AuthRequired(handlePageCreate))
	mux.HandleFunc("GET /admin/pages/{id}", AuthRequired(handlePageView))
	mux.HandleFunc("GET /admin/pages/{id}/edit", AuthRequired(handlePageEditForm))
	mux.HandleFunc("POST /admin/pages/{id}/update", AuthRequired(handlePageUpdate))
	mux.HandleFunc("POST /admin/pages/{id}/delete", AuthRequired(handlePageDelete))

	// Articles
	mux.HandleFunc("GET /admin/articles", AuthRequired(handleAdminArticles))
	mux.HandleFunc("GET /admin/articles/new", AuthRequired(handleArticleNewForm))
	mux.HandleFunc("POST /admin/articles", AuthRequired(handleArticleCreate))
	mux.HandleFunc("GET /admin/articles/{id}", AuthRequired(handleArticleView))
	mux.HandleFunc("GET /admin/articles/{id}/edit", AuthRequired(handleArticleEditForm))
	mux.HandleFunc("POST /admin/articles/{id}/update", AuthRequired(handleArticleUpdate))
	mux.HandleFunc("POST /admin/articles/{id}/delete", AuthRequired(handleArticleDelete))

	// Media
	mux.HandleFunc("GET /admin/media", AuthRequired(handleAdminMedia))
	mux.HandleFunc("POST /admin/media/upload", AuthRequired(handleMediaUpload))
	mux.HandleFunc("GET /admin/media/{id}/edit", AuthRequired(handleMediaEditForm))
	mux.HandleFunc("POST /admin/media/{id}/update", AuthRequired(handleMediaUpdate))
	mux.HandleFunc("POST /admin/media/{id}/delete", AuthRequired(handleMediaDelete))

	// Contact
	mux.HandleFunc("GET /admin/contact", AuthRequired(handleAdminContact))
	mux.HandleFunc("POST /admin/contact", AuthRequired(handleContactUpdate))

	port := os.Getenv("ADMIN_PORT")
	if port == "" {
		port = "8002"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("admin app listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID")
	html := `
<!DOCTYPE html>
<html>
<head><title>Admin Dashboard</title></head>
<body>
	<h1>Admin Dashboard</h1>
	<p>Logged in as user ID: %v</p>
	<ul>
		<li><a href="/admin/pages">Pages</a></li>
		<li><a href="/admin/articles">Articles</a></li>
		<li><a href="/admin/media">Media</a></li>
		<li><a href="/admin/contact">Contact Info</a></li>
	</ul>
	<form action="/logout" method="POST">
		<button type="submit">Log out</button>
	</form>
</body>
</html>`
	fmt.Fprintf(w, html, userID)
}

func runCreateUser(ctx context.Context) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)
	if username == "" {
		log.Fatal("username cannot be empty")
	}

	password := promptPassword()

	if err := CreateUser(ctx, username, password); err != nil {
		log.Fatalf("failed to create user: %v", err)
	}

	fmt.Printf("User %q created successfully.\n", username)
}

func runResetPassword(ctx context.Context) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)
	if username == "" {
		log.Fatal("username cannot be empty")
	}

	password := promptPassword()

	if err := UpdateUserPassword(ctx, username, password); err != nil {
		log.Fatalf("failed to reset password: %v", err)
	}

	fmt.Printf("Password for %q updated successfully.\n", username)
}

func promptPassword() string {
	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		log.Fatalf("failed to read password: %v", err)
	}
	password := string(passwordBytes)
	if len(password) < 12 {
		log.Fatal("password must be at least 12 characters")
	}

	fmt.Print("Confirm password: ")
	confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		log.Fatalf("failed to read password confirmation: %v", err)
	}
	if password != string(confirmBytes) {
		log.Fatal("passwords do not match")
	}

	return password
}
