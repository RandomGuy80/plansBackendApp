package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"plans-backend/internal/db"
	"plans-backend/internal/middleware"
	"plans-backend/internal/models"
)

// GET /me
func GetMe(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	var u models.User
	err := db.DB.QueryRowx(`
		SELECT id, phone, username, telegram_chat_id, created_at FROM users WHERE id = $1
	`, userID).StructScan(&u)
	if err != nil {
		jsonErr(w, "user not found", http.StatusNotFound)
		return
	}
	jsonOK(w, u)
}

// PATCH /me
func UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	var body struct {
		Username *string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "invalid body", http.StatusBadRequest)
		return
	}

	var u models.User
	err := db.DB.QueryRowx(`
		UPDATE users SET username = COALESCE($1, username)
		WHERE id = $2
		RETURNING id, phone, username, telegram_chat_id, created_at
	`, body.Username, userID).StructScan(&u)
	if err != nil {
		jsonErr(w, "update failed", http.StatusInternalServerError)
		return
	}
	jsonOK(w, u)
}

// GET /users/{username}
func GetUserByUsername(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")

	var u models.User
	err := db.DB.QueryRowx(`
		SELECT id, phone, username, telegram_chat_id, created_at FROM users WHERE username = $1
	`, username).StructScan(&u)
	if err != nil {
		jsonErr(w, "user not found", http.StatusNotFound)
		return
	}
	jsonOK(w, u)
}

// DELETE /me
func DeleteMe(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	// Каскадно удалятся meetings и meeting_members (ON DELETE CASCADE в миграции)
	_, err := db.DB.Exec(`DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		jsonErr(w, "delete failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
