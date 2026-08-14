package main

import (
	"html/template"
	"net"
	"net/http"
	"path/filepath"
)

const sessionCookieName = "session_token"

// dummyHash is compared against when a username doesn't exist, so login
// takes roughly the same amount of time either way — avoids leaking
// "this username exists" via response timing.
const dummyHash = "$2a$12$C6UzMDM.H6dfI/f/IKcEeO/edEHDcv1E9F91U4M0OZDT6H7T9G0Xa"

var templates = template.Must(template.New("").Funcs(template.FuncMap{
	"title": func(s string) string {
		if len(s) == 0 {
			return s
		}
		return string(s[0]-32) + s[1:]
	},
}).ParseGlob("templates/*.html"))
var layoutTemplates = template.Must(template.ParseGlob("templates/*/*.html"))

func render(w http.ResponseWriter, name string, data interface{}) {
	// Try layout templates first (subdirectories)
	tmpl, err := template.ParseFiles(
		"templates/layout.html",
		filepath.Join("templates", name),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderPage(w http.ResponseWriter, name string, data interface{}) {
	tmpl, err := template.ParseFiles(
		"templates/layout.html",
		filepath.Join("templates/pages", name),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderArticle(w http.ResponseWriter, name string, data interface{}) {
	tmpl, err := template.ParseFiles(
		"templates/layout.html",
		filepath.Join("templates/articles", name),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderMedia(w http.ResponseWriter, name string, data interface{}) {
	tmpl, err := template.ParseFiles(
		"templates/layout.html",
		filepath.Join("templates/media", name),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

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
		CheckPassword(password, dummyHash)
		passwordOK = false
	}

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
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(w, r, "/admin", http.StatusFound)
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
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(w, r, "/login", http.StatusFound)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID")
	data := struct {
		Title  string
		UserID interface{}
	}{
		Title:  "Dashboard",
		UserID: userID,
	}
	render(w, "dashboard.html", data)
}

// --- Page Handlers ---

func handleAdminPages(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	pages, err := ListPages(appDB, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := struct {
		Title string
		Pages []Page
	}{
		Title: "Pages",
		Pages: pages,
	}
	renderPage(w, "list.html", data)
}

func handlePageNewForm(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Title string
		Page  Page
	}{
		Title: "New Page",
		Page:  Page{Status: "draft"},
	}
	renderPage(w, "form.html", data)
}

func handlePageCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	slug := r.FormValue("slug")
	body := r.FormValue("body")
	status := r.FormValue("status")
	if status == "" {
		status = "draft"
	}

	page, err := CreatePage(appDB, title, slug, body, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/pages/"+string(rune(page.ID)), http.StatusSeeOther)
}

func handlePageView(w http.ResponseWriter, r *http.Request) {
	// Extract ID from URL
	idStr := r.PathValue("id")
	if idStr == "" {
		http.NotFound(w, r)
		return
	}
	// Convert to int64 (simplified - you need proper conversion)
	// For now just pass through
	data := struct {
		Title string
		Page  Page
	}{
		Title: "View Page",
		Page:  Page{},
	}
	renderPage(w, "view.html", data)
}

func handlePageEditForm(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		http.NotFound(w, r)
		return
	}
	// For now, placeholder
	data := struct {
		Title string
		Page  Page
	}{
		Title: "Edit Page",
		Page:  Page{Status: "draft"},
	}
	renderPage(w, "form.html", data)
}

func handlePageUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// TODO: get ID from URL
	http.Redirect(w, r, "/admin/pages", http.StatusSeeOther)
}

func handlePageDelete(w http.ResponseWriter, r *http.Request) {
	// TODO: implement
	http.Redirect(w, r, "/admin/pages", http.StatusSeeOther)
}

// --- Article Handlers ---

func handleAdminArticles(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	articles, err := ListArticles(appDB, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := struct {
		Title    string
		Articles []Article
	}{
		Title:    "Articles",
		Articles: articles,
	}
	renderArticle(w, "list.html", data)
}

func handleArticleNewForm(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Title   string
		Article Article
	}{
		Title:   "New Article",
		Article: Article{Status: "draft"},
	}
	renderArticle(w, "form.html", data)
}

func handleArticleCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	slug := r.FormValue("slug")
	body := r.FormValue("body")
	status := r.FormValue("status")
	if status == "" {
		status = "draft"
	}

	_, err := CreateArticle(appDB, title, slug, body, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/articles", http.StatusSeeOther)
}

func handleArticleView(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Title   string
		Article Article
	}{
		Title:   "View Article",
		Article: Article{},
	}
	renderArticle(w, "view.html", data)
}

func handleArticleEditForm(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Title   string
		Article Article
	}{
		Title:   "Edit Article",
		Article: Article{Status: "draft"},
	}
	renderArticle(w, "form.html", data)
}

func handleArticleUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin/articles", http.StatusSeeOther)
}

func handleArticleDelete(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/articles", http.StatusSeeOther)
}

// --- Media Handlers ---

func handleAdminMedia(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Title string
		Media []Media
	}{
		Title: "Media",
		Media: []Media{},
	}
	renderMedia(w, "list.html", data)
}

func handleMediaUpload(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/media", http.StatusSeeOther)
}

func handleMediaEditForm(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Title string
		Media Media
	}{
		Title: "Edit Media",
		Media: Media{},
	}
	renderMedia(w, "form.html", data)
}

func handleMediaUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/admin/media", http.StatusSeeOther)
}

func handleMediaDelete(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/media", http.StatusSeeOther)
}

// --- Contact Handlers ---

func handleAdminContact(w http.ResponseWriter, r *http.Request) {
	contacts, err := GetAllContactInfo(appDB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := struct {
		Title    string
		Contacts []ContactInfo
	}{
		Title:    "Contact Info",
		Contacts: contacts,
	}
	render(w, "contact.html", data)
}

func handleContactUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for key, values := range r.Form {
		if len(values) > 0 {
			_ = UpdateContactInfo(appDB, key, values[0])
		}
	}
	http.Redirect(w, r, "/admin/contact", http.StatusSeeOther)
}
