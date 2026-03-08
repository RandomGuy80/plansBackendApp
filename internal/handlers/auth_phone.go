package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"plans-backend/internal/auth"
	"plans-backend/internal/db"
	"plans-backend/internal/models"
	"plans-backend/internal/otp"
	"plans-backend/internal/telegram"
)

// phoneE164 validates E.164: +<1-3 country digits><6-12 local digits>
var phoneE164 = regexp.MustCompile(`^\+[1-9]\d{6,14}$`)

// ── Package-level dependencies (injected from main.go) ───────────────────────

var (
	OTPStore    *otp.Store
	TelegramBot *telegram.Bot
	PhoneChatIDs = &chatIDStore{}
)

// ── Request / response types ──────────────────────────────────────────────────

type sendCodeReq struct {
	Phone string `json:"phone"`
}

type verifyCodeReq struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

type authResponse struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

// ── Step 1: POST /auth/phone/send ─────────────────────────────────────────────
//
// Validates phone, checks that the number is linked to a Telegram chat,
// generates a 6-digit OTP and sends it via the bot.

func AuthPhoneSend(w http.ResponseWriter, r *http.Request) {
	var req sendCodeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", http.StatusBadRequest)
		return
	}

	phone := telegram.NormalizePhone(req.Phone)
	if !phoneE164.MatchString(phone) {
		jsonErr(w, "invalid phone number — use E.164 format, e.g. +79991234567", http.StatusBadRequest)
		return
	}

	chatID, linked := PhoneChatIDs.Get(phone)
	if !linked {
		jsonErr(w,
			"phone not linked to Telegram — open our bot and send: /start "+phone,
			http.StatusUnprocessableEntity,
		)
		return
	}

	code, err := OTPStore.Generate(phone)
	if err != nil {
		log.Printf("[auth] otp generate error: %v", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}

	msg := "🔐 Ваш код входа: *" + code + "*\n\nДействителен 5 минут. Никому не сообщайте."
	if err := TelegramBot.SendMessage(chatID, msg); err != nil {
		log.Printf("[auth] telegram send error phone=%s chat_id=%d: %v", phone, chatID, err)
		jsonErr(w, "failed to send code via Telegram", http.StatusInternalServerError)
		return
	}

	log.Printf("[auth] OTP sent phone=%s chat_id=%d", phone, chatID)
	jsonOK(w, map[string]string{"message": "code sent to your Telegram"})
}

// ── Step 2: POST /auth/phone/verify ──────────────────────────────────────────
//
// Verifies the OTP, then finds or creates the user in the DB,
// updates telegram_chat_id, and returns a JWT.

func AuthPhoneVerify(w http.ResponseWriter, r *http.Request) {
	var req verifyCodeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", http.StatusBadRequest)
		return
	}

	phone := telegram.NormalizePhone(req.Phone)
	if !phoneE164.MatchString(phone) {
		jsonErr(w, "invalid phone number", http.StatusBadRequest)
		return
	}

	code := strings.TrimSpace(req.Code)
	if len(code) == 0 {
		jsonErr(w, "code is required", http.StatusBadRequest)
		return
	}

	if !OTPStore.Verify(phone, code) {
		jsonErr(w, "invalid or expired code", http.StatusUnauthorized)
		return
	}

	handlePhoneAuth(w, phone)
}

// handlePhoneAuth finds or creates a user by phone, links telegram_chat_id,
// and writes a JWT response. Called after successful OTP verification.
func handlePhoneAuth(w http.ResponseWriter, phone string) {
	chatID, _ := PhoneChatIDs.Get(phone)

	// Upsert user: insert if not exists, always update telegram_chat_id
	var user models.User
	err := db.DB.QueryRowx(`
		INSERT INTO users (phone, telegram_chat_id)
		VALUES ($1, $2)
		ON CONFLICT (phone) DO UPDATE
		  SET telegram_chat_id = EXCLUDED.telegram_chat_id
		RETURNING id, phone, username, telegram_chat_id, created_at
	`, phone, chatID).StructScan(&user)
	if err != nil {
		log.Printf("[auth] db upsert error phone=%s: %v", phone, err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}

	token, err := auth.GenerateToken(user.ID)
	if err != nil {
		log.Printf("[auth] token generate error: %v", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}

	jsonOK(w, authResponse{Token: token, User: &user})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// ── chatIDStore ───────────────────────────────────────────────────────────────
// Thread-safe in-memory phone → chat_id map.
// Populated when user sends /start <phone> to the Telegram bot.
//
// ⚠️  For production with multiple instances or restarts:
//     replace Get/Set with DB queries on the users.telegram_chat_id column.

type chatIDStore struct {
	mu sync.RWMutex
	m  map[string]int64
}

func (s *chatIDStore) Get(phone string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.m == nil {
		return 0, false
	}
	id, ok := s.m[phone]
	return id, ok
}

func (s *chatIDStore) Set(phone string, chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = make(map[string]int64)
	}
	s.m[phone] = chatID
}
