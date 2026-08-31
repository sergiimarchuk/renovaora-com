package handlers

import (
        "html/template"
        "log"
        "net/http"
)

// HomeHandler рендерит главную страницу через шаблоны base.html + index.html
func HomeHandler(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/" {
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

        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        err = ts.ExecuteTemplate(w, "base", nil)
        if err != nil {
                log.Println(err)
                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        }
}
