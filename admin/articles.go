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

type Article struct {
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

func CreateArticle(db *sql.DB, title, slug, body, status string) (*Article, error) {
	slug = generateSlug(slug)
	query := `
		INSERT INTO articles (title, slug, body_markdown, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`
	article := &Article{Title: title, Slug: slug, BodyMarkdown: body, Status: status}
	err := db.QueryRow(query, title, slug, body, status).Scan(&article.ID, &article.CreatedAt, &article.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return article, nil
}

func GetArticleByID(db *sql.DB, id int64) (*Article, error) {
	query := `
		SELECT id, title, slug, body_markdown, status, created_at, updated_at, published_at, deleted_at
		FROM articles WHERE id = $1 AND deleted_at IS NULL
	`
	article := &Article{}
	err := db.QueryRow(query, id).Scan(
		&article.ID, &article.Title, &article.Slug, &article.BodyMarkdown,
		&article.Status, &article.CreatedAt, &article.UpdatedAt,
		&article.PublishedAt, &article.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return article, err
}

func GetArticleBySlug(db *sql.DB, slug string) (*Article, error) {
	query := `
		SELECT id, title, slug, body_markdown, status, created_at, updated_at, published_at, deleted_at
		FROM articles WHERE slug = $1 AND deleted_at IS NULL AND status = 'published'
	`
	article := &Article{}
	err := db.QueryRow(query, slug).Scan(
		&article.ID, &article.Title, &article.Slug, &article.BodyMarkdown,
		&article.Status, &article.CreatedAt, &article.UpdatedAt,
		&article.PublishedAt, &article.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return article, err
}

func UpdateArticle(db *sql.DB, id int64, title, slug, body, status string) error {
	slug = generateSlug(slug)
	query := `
		UPDATE articles
		SET title = $1, slug = $2, body_markdown = $3, status = $4, updated_at = NOW(),
		    published_at = CASE WHEN $4 = 'published' AND published_at IS NULL THEN NOW() ELSE published_at END
		WHERE id = $5 AND deleted_at IS NULL
	`
	_, err := db.Exec(query, title, slug, body, status, id)
	return err
}

func DeleteArticle(db *sql.DB, id int64) error {
	query := `UPDATE articles SET deleted_at = NOW() WHERE id = $1`
	_, err := db.Exec(query, id)
	return err
}

func ListArticles(db *sql.DB, status string) ([]Article, error) {
	query := `
		SELECT id, title, slug, body_markdown, status, created_at, updated_at, published_at, deleted_at
		FROM articles
		WHERE ($1 = '' OR status = $1) AND deleted_at IS NULL
		ORDER BY created_at DESC
	`
	rows, err := db.Query(query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var a Article
		err := rows.Scan(&a.ID, &a.Title, &a.Slug, &a.BodyMarkdown,
			&a.Status, &a.CreatedAt, &a.UpdatedAt,
			&a.PublishedAt, &a.DeletedAt)
		if err != nil {
			return nil, err
		}
		articles = append(articles, a)
	}
	return articles, rows.Err()
}

func ListPublishedArticles(db *sql.DB, limit int) ([]Article, error) {
	query := `
		SELECT id, title, slug, body_markdown, status, created_at, updated_at, published_at, deleted_at
		FROM articles
		WHERE status = 'published' AND deleted_at IS NULL
		ORDER BY published_at DESC
		LIMIT $1
	`
	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var a Article
		err := rows.Scan(&a.ID, &a.Title, &a.Slug, &a.BodyMarkdown,
			&a.Status, &a.CreatedAt, &a.UpdatedAt,
			&a.PublishedAt, &a.DeletedAt)
		if err != nil {
			return nil, err
		}
		articles = append(articles, a)
	}
	return articles, rows.Err()
}

// --- HTTP Обработчики ---

func handleAdminArticles(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/admin/articles" {
		handleArticlesList(w, r)
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/admin/articles/new") {
		handleArticleNewForm(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/admin/articles" {
		handleArticleCreate(w, r)
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/admin/articles/") {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) == 4 && parts[3] == "edit" {
			handleArticleEditForm(w, r)
			return
		}
		if len(parts) == 3 {
			handleArticleView(w, r)
			return
		}
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/admin/articles/") {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) == 4 && parts[3] == "delete" {
			handleArticleDelete(w, r)
			return
		}
		if len(parts) == 4 && parts[3] == "update" {
			handleArticleUpdate(w, r)
			return
		}
	}
	http.NotFound(w, r)
}

func handleArticlesList(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	articles, err := ListArticles(appDB, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// TODO: render template
	w.Write([]byte("Articles list - coming soon"))
}

func handleArticleNewForm(w http.ResponseWriter, r *http.Request) {
	// TODO: render form
	w.Write([]byte("New article form - coming soon"))
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

	article, err := CreateArticle(appDB, title, slug, body, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/articles/%d", article.ID), http.StatusSeeOther)
}

func handleArticleView(w http.ResponseWriter, r *http.Request) {
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
	article, err := GetArticleByID(appDB, id)
	if err != nil || article == nil {
		http.NotFound(w, r)
		return
	}
	// TODO: render template
	w.Write([]byte("Article view - coming soon"))
}

func handleArticleEditForm(w http.ResponseWriter, r *http.Request) {
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
	article, err := GetArticleByID(appDB, id)
	if err != nil || article == nil {
		http.NotFound(w, r)
		return
	}
	// TODO: render template
	w.Write([]byte("Edit article form - coming soon"))
}

func handleArticleUpdate(w http.ResponseWriter, r *http.Request) {
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

	err = UpdateArticle(appDB, id, title, slug, body, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/articles/%d", id), http.StatusSeeOther)
}

func handleArticleDelete(w http.ResponseWriter, r *http.Request) {
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
	err = DeleteArticle(appDB, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/articles", http.StatusSeeOther)
}
