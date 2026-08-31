package main

import (
        "log"
        "net/http"

        "renovaora.com/internal/handlers"
)

func main() {
        mux := http.NewServeMux()

        // Раздача статических файлов (css/js/картинки) из ui/static
        fileServer := http.FileServer(http.Dir("./ui/static/"))
        mux.Handle("/static/", http.StripPrefix("/static", fileServer))

        // Регистрация маршрута главной страницы
        mux.HandleFunc("/", handlers.HomeHandler)

        log.Println("Server successfully started on port :8080 (http://localhost:8080)")
        err := http.ListenAndServe(":8080", mux)
        if err != nil {
                log.Fatal(err)
        }
}
