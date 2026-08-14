package main

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// --- Модель ---

type Media struct {
	ID         int64
	Filename   string
	Path       string
	AltText    string
	Caption    string
	EntityType string // 'page' or 'article'
	EntityID   int64
	CreatedAt  time.Time
	DeletedAt  *time.Time
}

// --- Функции БД ---

func CreateMedia(db *sql.DB, filename, path, altText, caption, entityType string, entityID int64) (*Media, error) {
	query := `
		INSERT INTO media (filename, path, alt_text, caption, entity_type, entity_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`
	media := &Media{
		Filename:   filename,
		Path:       path,
		AltText:    altText,
		Caption:    caption,
		EntityType: entityType,
		EntityID:   entityID,
	}
	err := db.QueryRow(query, filename, path, altText, caption, entityType, entityID).Scan(&media.ID, &media.CreatedAt)
	if err != nil {
		return nil, err
	}
	return media, nil
}

func GetMediaByID(db *sql.DB, id int64) (*Media, error) {
	query := `
		SELECT id, filename, path, alt_text, caption, entity_type, entity_id, created_at, deleted_at
		FROM media WHERE id = $1 AND deleted_at IS NULL
	`
	media := &Media{}
	err := db.QueryRow(query, id).Scan(
		&media.ID, &media.Filename, &media.Path, &media.AltText,
		&media.Caption, &media.EntityType, &media.EntityID,
		&media.CreatedAt, &media.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return media, err
}

func GetMediaByEntity(db *sql.DB, entityType string, entityID int64) ([]Media, error) {
	query := `
		SELECT id, filename, path, alt_text, caption, entity_type, entity_id, created_at, deleted_at
		FROM media
		WHERE entity_type = $1 AND entity_id = $2 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`
	rows, err := db.Query(query, entityType, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mediaList []Media
	for rows.Next() {
		var m Media
		err := rows.Scan(&m.ID, &m.Filename, &m.Path, &m.AltText,
			&m.Caption, &m.EntityType, &m.EntityID,
			&m.CreatedAt, &m.DeletedAt)
		if err != nil {
			return nil, err
		}
		mediaList = append(mediaList, m)
	}
	return mediaList, rows.Err()
}

func UpdateMedia(db *sql.DB, id int64, altText, caption string) error {
	query := `UPDATE media SET alt_text = $1, caption = $2 WHERE id = $3 AND deleted_at IS NULL`
	_, err := db.Exec(query, altText, caption, id)
	return err
}

func DeleteMedia(db *sql.DB, id int64) error {
	// Get path first
	var path string
	err := db.QueryRow(`SELECT path FROM media WHERE id = $1`, id).Scan(&path)
	if err != nil {
		return err
	}

	// Soft delete in DB
	_, err = db.Exec(`UPDATE media SET deleted_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return err
	}

	// Delete file from staging
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// --- HTTP Обработчики ---

func handleAdminMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/admin/media" {
		handleMediaList(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/admin/media/upload" {
		handleMediaUpload(w, r)
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/admin/media/") {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) == 4 && parts[3] == "delete" {
			handleMediaDelete(w, r)
			return
		}
		if len(parts) == 4 && parts[3] == "edit" {
			handleMediaEditForm(w, r)
			return
		}
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/admin/media/") {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) == 4 && parts[3] == "update" {
			handleMediaUpdate(w, r)
			return
		}
	}
	http.NotFound(w, r)
}

func handleMediaList(w http.ResponseWriter, r *http.Request) {
	entityType := r.URL.Query().Get("type")
	entityID := r.URL.Query().Get("id")
	if entityType != "" && entityID != "" {
		id, err := strconv.ParseInt(entityID, 10, 64)
		if err == nil {
			mediaList, err := GetMediaByEntity(appDB, entityType, id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// TODO: render template
			w.Write([]byte(fmt.Sprintf("Media list for %s %d - coming soon", entityType, id)))
			return
		}
	}
	w.Write([]byte("Media list - coming soon"))
}

func handleMediaUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB limit
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	entityType := r.FormValue("entity_type")
	entityIDStr := r.FormValue("entity_id")
	altText := r.FormValue("alt_text")
	caption := r.FormValue("caption")

	entityID, err := strconv.ParseInt(entityIDStr, 10, 64)
	if err != nil || (entityType != "page" && entityType != "article") {
		http.Error(w, "Invalid entity type or ID", http.StatusBadRequest)
		return
	}

	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), generateSlug(strings.TrimSuffix(header.Filename, ext)), ext)

	// Save to staging
	stagingPath := "/media/staging/" + filename
	out, err := os.Create(stagingPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Save to DB
	media, err := CreateMedia(appDB, header.Filename, stagingPath, altText, caption, entityType, entityID)
	if err != nil {
		os.Remove(stagingPath)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// TODO: Return JSON or redirect
	w.Write([]byte(fmt.Sprintf("Uploaded: %s (ID: %d)", media.Filename, media.ID)))
}

func handleMediaEditForm(w http.ResponseWriter, r *http.Request) {
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
	media, err := GetMediaByID(appDB, id)
	if err != nil || media == nil {
		http.NotFound(w, r)
		return
	}
	// TODO: render template
	w.Write([]byte("Edit media form - coming soon"))
}

func handleMediaUpdate(w http.ResponseWriter, r *http.Request) {
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
	altText := r.FormValue("alt_text")
	caption := r.FormValue("caption")

	err = UpdateMedia(appDB, id, altText, caption)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/media", http.StatusSeeOther)
}

func handleMediaDelete(w http.ResponseWriter, r *http.Request) {
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
	err = DeleteMedia(appDB, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/media", http.StatusSeeOther)
}

func MoveMediaToLive(db *sql.DB, mediaID int64) error {
	media, err := GetMediaByID(db, mediaID)
	if err != nil || media == nil {
		return errors.New("media not found")
	}

	// Move file from staging to live
	stagingPath := media.Path
	livePath := strings.Replace(stagingPath, "/staging/", "/live/", 1)

	// Ensure live directory exists
	if err := os.MkdirAll(filepath.Dir(livePath), 0755); err != nil {
		return err
	}

	if err := os.Rename(stagingPath, livePath); err != nil {
		return err
	}

	// Update DB with new path
	_, err = db.Exec(`UPDATE media SET path = $1 WHERE id = $2`, livePath, mediaID)
	return err
}
