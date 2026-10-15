package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strings"

	"renovaora.com/internal/i18n"
)

type PageData struct {
	Lang string
	T    map[string]string
}

// HomeHandler рендерит главную страницу для данного языка.
// Если запрошен путь глубже языкового префикса (например /en/services,
// которого ещё не существует), делаем редирект на главную того же языка
// вместо 404 — сайт ещё наполняется, посетителю не нужно видеть "битую" страницу.
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	lang := extractLang(r.URL.Path)

	rest := strings.TrimPrefix(r.URL.Path, "/"+lang)
	rest = strings.Trim(rest, "/")

	if rest != "" {
		http.Redirect(w, r, "/"+lang+"/", http.StatusFound)
		return
	}

	files := []string{
		"./ui/html/layouts/base.html",
		"./ui/html/pages/index.html",
	}

	ts, err := template.ParseFiles(files...)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := PageData{
		Lang: lang,
		T:    i18n.T(lang),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ts.ExecuteTemplate(w, "base", data); err != nil {
		log.Println(err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// RootRedirect ловит "/" и любой путь, не попавший под /en/, /it/, /de/, /static/
// (ServeMux отправляет их сюда как в catch-all "/"). Вместо 404 — редирект
// на главную страницу языка по умолчанию.
func RootRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/"+i18n.DefaultLang+"/", http.StatusFound)
}

func extractLang(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return i18n.DefaultLang
	}
	candidate := parts[0]
	if i18n.IsSupported(candidate) {
		return candidate
	}
	return i18n.DefaultLang
}
