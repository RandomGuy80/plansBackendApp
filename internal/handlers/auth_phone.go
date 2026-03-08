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

// phoneBasic: минимум 6 символов, только цифры/+/-/пробел
var phoneBasic = regexp.MustCompile(`^[\d\s\+\-\(\)]{6,20}$`)

// ── Зависимости (устанавливаются в main.go) ───────────────────────────────────

var (
	OTPStore     *otp.Store
	TelegramBot  *telegram.Bot
	PhoneChatIDs = &chatIDStore{} // резервный in-memory fallback
)

// ── Типы запросов ─────────────────────────────────────────────────────────────

type sendCodeReq struct {
	Phone      string `json:"phone"`
	Name       string `json:"name"`
	TelegramID int64  `json:"telegram_id"`
}

type verifyCodeReq struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
	Name  string `json:"name"`
}

// ── POST /auth/send-code ──────────────────────────────────────────────────────
//
// Принимает phone + name + telegram_id (chat_id бота).
// Валидирует номер, сохраняет chat_id, генерирует OTP и отправляет в Telegram.

func AuthSendCode(w http.ResponseWriter, r *http.Request) {
	var req sendCodeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", http.StatusBadRequest)
		return
	}

	phone := normalizePhone(req.Phone)
	if !phoneBasic.MatchString(phone) {
		jsonErr(w, "введи корректный номер телефона", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		jsonErr(w, "введи своё имя", http.StatusBadRequest)
		return
	}

	if req.TelegramID == 0 {
		jsonErr(w, "введи Telegram ID", http.StatusBadRequest)
		return
	}

	// Сохраняем chat_id для этого номера
	PhoneChatIDs.Set(phone, req.TelegramID)

	code, err := OTPStore.Generate(phone)
	if err != nil {
		log.Printf("[auth] otp generate: %v", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}

	msg := "🔐 Ваш код входа в Plans: *" + code + "*\n\nДействителен 5 минут. Никому не сообщайте."
	if err := TelegramBot.SendMessage(req.TelegramID, msg); err != nil {
		log.Printf("[auth] telegram send error phone=%s chat_id=%d: %v", phone, req.TelegramID, err)
		jsonErr(w, "не удалось отправить код в Telegram — проверь правильность Telegram ID", http.StatusUnprocessableEntity)
		return
	}

	log.Printf("[auth] OTP sent phone=%s chat_id=%d", phone, req.TelegramID)
	jsonOK(w, map[string]string{"message": "код отправлен в Telegram"})
}

// ── POST /auth/verify-code ────────────────────────────────────────────────────
//
// Проверяет OTP, создаёт/находит пользователя, возвращает JWT.

func AuthVerifyCode(w http.ResponseWriter, r *http.Request) {
	var req verifyCodeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid request body", http.StatusBadRequest)
		return
	}

	phone := normalizePhone(req.Phone)
	if !phoneBasic.MatchString(phone) {
		jsonErr(w, "invalid phone number", http.StatusBadRequest)
		return
	}

	code := strings.TrimSpace(req.Code)
	if len(code) == 0 {
		jsonErr(w, "code is required", http.StatusBadRequest)
		return
	}

	if !OTPStore.Verify(phone, code) {
		jsonErr(w, "неверный или устаревший код", http.StatusUnauthorized)
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Пользователь"
	}

	chatID, _ := PhoneChatIDs.Get(phone)
	upsertAndRespond(w, phone, name, chatID)
}

// upsertAndRespond — создаёт или обновляет пользователя в БД, возвращает JWT
func upsertAndRespond(w http.ResponseWriter, phone, name string, chatID int64) {
	var user models.User
	err := db.DB.QueryRowx(`
		INSERT INTO users (phone, username, telegram_chat_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (phone) DO UPDATE
		  SET username          = COALESCE(EXCLUDED.username, users.username),
		      telegram_chat_id  = COALESCE(NULLIF(EXCLUDED.telegram_chat_id, 0), users.telegram_chat_id)
		RETURNING id, phone, username, telegram_chat_id, created_at
	`, phone, name, chatID).StructScan(&user)
	if err != nil {
		log.Printf("[auth] db upsert phone=%s: %v", phone, err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}

	token, err := auth.GenerateToken(user.ID)
	if err != nil {
		log.Printf("[auth] generate token: %v", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Фронт ожидает поле "name" в user — маппим из username
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

// ── Helpers ───────────────────────────────────────────────────────────────────

func normalizePhone(raw string) string {
	raw = strings.TrimSpace(raw)
	// Убираем всё кроме цифр и +
	var b strings.Builder
	for i, ch := range raw {
		if ch == '+' && i == 0 {
			b.WriteRune(ch)
		} else if ch >= '0' && ch <= '9' {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

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
