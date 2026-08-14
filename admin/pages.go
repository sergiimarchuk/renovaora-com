package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// --- Модель ---

type Page struct {
	ID           int64
	Title        string
	Slug         string
	BodyMarkdown string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	PublishedAt  *time.Time
	DeletedAt    *time.Time
}

// --- Функции БД ---

func CreatePage(db *sql.DB, title, slug, body, status string) (*Page, error) {
	slug = generateSlug(slug)
	query := `
		INSERT INTO pages (title, slug, body_markdown, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`
	page := &Page{Title: title, Slug: slug, BodyMarkdown: body, Status: status}
	err := db.QueryRow(query, title, slug, body, status).Scan(&page.ID, &page.CreatedAt, &page.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return page, nil
}

func GetPageByID(db *sql.DB, id int64) (*Page, error) {
	query := `
		SELECT id, title, slug, body_markdown, status, created_at, updated_at, published_at, deleted_at
		FROM pages WHERE id = $1 AND deleted_at IS NULL
	`
	page := &Page{}
	err := db.QueryRow(query, id).Scan(
		&page.ID, &page.Title, &page.Slug, &page.BodyMarkdown,
		&page.Status, &page.CreatedAt, &page.UpdatedAt,
		&page.PublishedAt, &page.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return page, err
}

func GetPageBySlug(db *sql.DB, slug string) (*Page, error) {
	query := `
		SELECT id, title, slug, body_markdown, status, created_at, updated_at, published_at, deleted_at
		FROM pages WHERE slug = $1 AND deleted_at IS NULL
	`
	page := &Page{}
	err := db.QueryRow(query, slug).Scan(
		&page.ID, &page.Title, &page.Slug, &page.BodyMarkdown,
		&page.Status, &page.CreatedAt, &page.UpdatedAt,
		&page.PublishedAt, &page.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return page, err
}

func UpdatePage(db *sql.DB, id int64, title, slug, body, status string) error {
	slug = generateSlug(slug)
	query := `
		UPDATE pages
		SET title = $1, slug = $2, body_markdown = $3, status = $4, updated_at = NOW(),
		    published_at = CASE WHEN $4 = 'published' AND published_at IS NULL THEN NOW() ELSE published_at END
		WHERE id = $5 AND deleted_at IS NULL
	`
	_, err := db.Exec(query, title, slug, body, status, id)
	return err
}

func DeletePage(db *sql.DB, id int64) error {
	query := `UPDATE pages SET deleted_at = NOW() WHERE id = $1`
	_, err := db.Exec(query, id)
	return err
}

func ListPages(db *sql.DB, status string) ([]Page, error) {
	query := `
		SELECT id, title, slug, body_markdown, status, created_at, updated_at, published_at, deleted_at
		FROM pages
		WHERE ($1 = '' OR status = $1) AND deleted_at IS NULL
		ORDER BY created_at DESC
	`
	rows, err := db.Query(query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []Page
	for rows.Next() {
		var p Page
		err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.BodyMarkdown,
			&p.Status, &p.CreatedAt, &p.UpdatedAt,
			&p.PublishedAt, &p.DeletedAt)
		if err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

// --- HTTP Обработчики ---

func handleAdminPages(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/admin/pages" {
		handlePagesList(w, r)
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/admin/pages/new") {
		handlePageNewForm(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/admin/pages" {
		handlePageCreate(w, r)
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/admin/pages/") {
		// /admin/pages/123/edit
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) == 4 && parts[3] == "edit" {
			handlePageEditForm(w, r)
			return
		}
		if len(parts) == 3 {
			handlePageView(w, r)
			return
		}
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/admin/pages/") {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) == 4 && parts[3] == "delete" {
			handlePageDelete(w, r)
			return
		}
		if len(parts) == 4 && parts[3] == "update" {
			handlePageUpdate(w, r)
			return
		}
	}
	http.NotFound(w, r)
}

func handlePagesList(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	pages, err := ListPages(appDB, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// TODO: render template
	w.Write([]byte("Pages list - coming soon"))
}

func handlePageNewForm(w http.ResponseWriter, r *http.Request) {
	// TODO: render form
	w.Write([]byte("New page form - coming soon"))
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
	http.Redirect(w, r, fmt.Sprintf("/admin/pages/%d", page.ID), http.StatusSeeOther)
}

func handlePageView(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	page, err := GetPageByID(appDB, id)
	if err != nil || page == nil {
		http.NotFound(w, r)
		return
	}
	// TODO: render template
	w.Write([]byte("Page view - coming soon"))
}

func handlePageEditForm(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	page, err := GetPageByID(appDB, id)
	if err != nil || page == nil {
		http.NotFound(w, r)
		return
	}
	// TODO: render template
	w.Write([]byte("Edit page form - coming soon"))
}

func handlePageUpdate(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
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

	err = UpdatePage(appDB, id, title, slug, body, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/pages/%d", id), http.StatusSeeOther)
}

func handlePageDelete(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	err = DeletePage(appDB, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/pages", http.StatusSeeOther)
}

func generateSlug(slug string) string {
	slug = strings.ToLower(slug)
	slug = strings.ReplaceAll(slug, " ", "-")
	// удалить спецсимволы
	return slug
}
