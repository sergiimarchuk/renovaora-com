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

	// Fail fast if the DB isn't actually reachable, rather than starting
	// the HTTP server and only discovering the problem on first request
	if err := db.Ping(ctx); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}
	log.Println("connected to auth database")

	// --create-user is a one-off CLI mode, not an HTTP route — this system
	// has no public registration by design, so this is the only way to
	// create an admin account. Running it exits immediately afterward
	// rather than starting the HTTP server.
	if len(os.Args) > 1 && os.Args[1] == "--create-user" {
		runCreateUser(ctx)
		return
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", handleHealthz)

	// Auth routes
	mux.HandleFunc("GET /login", handleLoginPage)
	mux.HandleFunc("POST /login", handleLoginSubmit)
	mux.HandleFunc("POST /logout", handleLogout)

	// Protected routes — content management handlers will be added here
	// in the next step, all wrapped in AuthRequired
	mux.HandleFunc("GET /", AuthRequired(handleDashboard))

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

// runCreateUser interactively prompts for a username and password on the
// terminal and inserts a new admin user. Intended to be run once via:
//   docker exec -it renovaora-admin /app/admin-app --create-user
func runCreateUser(ctx context.Context) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)
	if username == "" {
		log.Fatal("username cannot be empty")
	}

	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // newline since ReadPassword doesn't echo the Enter key
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

	if err := CreateUser(ctx, username, password); err != nil {
		log.Fatalf("failed to create user: %v", err)
	}

	fmt.Printf("User %q created successfully.\n", username)
}

