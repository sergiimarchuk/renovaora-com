package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strings"

	"renovaora.com/internal/i18n"
)

// PageData передаётся в шаблон.
type PageData struct {
	Lang string            // текущий язык, напр. "en"
	T    map[string]string // словарь переводов текущего языка
}

// HomeHandler рендерит главную страницу для конкретного языка.
// Ожидает путь вида /en/, /it/, /de/
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	lang := extractLang(r.URL.Path)

	// Путь после языкового префикса, напр. "/en/about" -> "about"
	rest := strings.TrimPrefix(r.URL.Path, "/"+lang)
	rest = strings.Trim(rest, "/")

	if rest != "" {
		// Пока обрабатываем только главную страницу для каждого языка.
		// Обработчики для /about, /services и т.д. добавляются сюда позже.
		http.NotFound(w, r)
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

// RootRedirect перенаправляет "/" на язык по умолчанию, напр. "/" -> "/en/".
func RootRedirect(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/"+i18n.DefaultLang+"/", http.StatusFound)
}

// extractLang достаёт код языка из первого сегмента пути.
// "/it/about" -> "it". Если сегмент невалиден — возвращает DefaultLang.
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
