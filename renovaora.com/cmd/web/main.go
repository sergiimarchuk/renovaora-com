package main

import (
	"log"
	"net/http"

	"renovaora.com/internal/handlers"
	"renovaora.com/internal/i18n"
)

func main() {
	// Загружаем переводы один раз при старте.
	if err := i18n.Load("./internal/i18n"); err != nil {
		log.Fatalf("failed to load translations: %v", err)
	}

	mux := http.NewServeMux()

	// Раздача статики (картинки, css, js)
	fileServer := http.FileServer(http.Dir("./ui/static/"))
	mux.Handle("/static/", http.StripPrefix("/static", fileServer))

	// "/" -> редирект на язык по умолчанию, напр. "/en/"
	mux.HandleFunc("/", handlers.RootRedirect)

	// Языковые маршруты
	mux.HandleFunc("/en/", handlers.HomeHandler)
	mux.HandleFunc("/it/", handlers.HomeHandler)
	mux.HandleFunc("/de/", handlers.HomeHandler)

	log.Println("Server successfully started on port :8080 (http://localhost:8080)")
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal(err)
	}
}
