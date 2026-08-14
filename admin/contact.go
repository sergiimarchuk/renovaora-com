package main

import (
	"database/sql"
	"errors"
	"net/http"
	"time"
)

// --- Модель ---

type ContactInfo struct {
	ID        int64
	Key       string
	Value     string
	UpdatedAt time.Time
}

// --- Функции БД ---

func GetContactInfo(db *sql.DB, key string) (*ContactInfo, error) {
	query := `SELECT id, key, value, updated_at FROM contact_info WHERE key = $1`
	contact := &ContactInfo{}
	err := db.QueryRow(query, key).Scan(&contact.ID, &contact.Key, &contact.Value, &contact.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return contact, err
}

func GetAllContactInfo(db *sql.DB) ([]ContactInfo, error) {
	query := `SELECT id, key, value, updated_at FROM contact_info ORDER BY key`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []ContactInfo
	for rows.Next() {
		var c ContactInfo
		err := rows.Scan(&c.ID, &c.Key, &c.Value, &c.UpdatedAt)
		if err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

func UpdateContactInfo(db *sql.DB, key, value string) error {
	query := `
		UPDATE contact_info 
		SET value = $1, updated_at = NOW() 
		WHERE key = $2
	`
	result, err := db.Exec(query, value, key)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		// Insert if not exists
		query = `INSERT INTO contact_info (key, value) VALUES ($1, $2)`
		_, err = db.Exec(query, key, value)
		return err
	}
	return nil
}

// --- HTTP Обработчики ---

func handleAdminContact(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/admin/contact" {
		handleContactList(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/admin/contact" {
		handleContactUpdate(w, r)
		return
	}
	http.NotFound(w, r)
}

func handleContactList(w http.ResponseWriter, r *http.Request) {
	contacts, err := GetAllContactInfo(appDB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// TODO: render template
	w.Write([]byte("Contact info - coming soon"))
}

func handleContactUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Update all fields
	for key, values := range r.Form {
		if len(values) > 0 {
			err := UpdateContactInfo(appDB, key, values[0])
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	http.Redirect(w, r, "/admin/contact", http.StatusSeeOther)
}
