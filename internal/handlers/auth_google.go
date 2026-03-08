package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"plans-backend/internal/auth"
	"plans-backend/internal/db"
	"plans-backend/internal/models"
)

type googleAuthReq struct {
	GoogleUID string `json:"google_uid"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	PhotoURL  string `json:"photo_url"`
	Username  string `json:"username"`
}

// POST /auth/google
func AuthGoogle(w http.ResponseWriter, r *http.Request) {
	var req googleAuthReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.GoogleUID) == "" {
		jsonErr(w, "google_uid is required", http.StatusBadRequest)
		return
	}

	var user models.User
	err := db.DB.QueryRowx(`
		INSERT INTO users (phone, username, telegram_chat_id)
		VALUES ($1, $2, 0)
		ON CONFLICT (phone) DO UPDATE
		  SET username = COALESCE(EXCLUDED.username, users.username)
		RETURNING id, phone, username, telegram_chat_id, created_at
	`, req.Email, req.Name).StructScan(&user)
	if err != nil {
		log.Printf("[google auth] db upsert: %v", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}

	token, err := auth.GenerateToken(user.ID)
	if err != nil {
		log.Printf("[google auth] generate token: %v", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}

	type userResp struct {
		ID    string  `json:"id"`
		Phone string  `json:"phone"`
		Name  *string `json:"name"`
	}
	jsonOK(w, map[string]interface{}{
		"token": token,
		"user": userResp{
			ID:    user.ID,
			Phone: user.Phone,
			Name:  user.Username,
		},
	})
}
