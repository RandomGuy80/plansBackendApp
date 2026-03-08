package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

const apiBase = "https://api.telegram.org/bot"

type Bot struct {
	token string
}

func NewBot(token string) *Bot {
	return &Bot{token: token}
}

func (b *Bot) SendMessage(chatID int64, text string) error {
	url := fmt.Sprintf("%s%s/sendMessage", apiBase, b.token)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	return nil
}

// Update represents an incoming Telegram update
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	Chat Chat   `json:"chat"`
	Text string `json:"text"`
	Contact *Contact `json:"contact"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type Contact struct {
	PhoneNumber string `json:"phone_number"`
	UserID      int64  `json:"user_id"`
}

// ParseStartPhone extracts phone number from /start <phone> command
// Returns normalized phone (digits only, with leading +) or empty string
func ParseStartPhone(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/start") {
		return ""
	}
	parts := strings.Fields(text)
	if len(parts) < 2 {
		return ""
	}
	return NormalizePhone(parts[1])
}

// NormalizePhone strips non-digit chars, ensures leading +
func NormalizePhone(raw string) string {
	var digits strings.Builder
	for _, ch := range raw {
		if ch >= '0' && ch <= '9' {
			digits.WriteRune(ch)
		}
	}
	d := digits.String()
	if len(d) < 7 || len(d) > 15 {
		return ""
	}

	// If original had + prefix keep it, otherwise add +
	if strings.HasPrefix(strings.TrimSpace(raw), "+") {
		return "+" + d
	}
	// Assume international if 10+ digits
	if len(d) >= 10 {
		return "+" + d
	}
	return ""
}

// WebhookHandler returns an http.HandlerFunc that processes Telegram webhook updates
// and calls onPhone(phone, chatID) when a user sends /start <phone>
func (b *Bot) WebhookHandler(onPhone func(phone string, chatID int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var update Update
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			log.Printf("telegram webhook decode error: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if update.Message == nil {
			w.WriteHeader(http.StatusOK)
			return
		}

		chatID := update.Message.Chat.ID

		// Handle /start <phone>
		phone := ParseStartPhone(update.Message.Text)
		if phone != "" {
			onPhone(phone, chatID)
			_ = b.SendMessage(chatID, "✅ Номер привязан! Теперь вы будете получать коды входа здесь.")
			w.WriteHeader(http.StatusOK)
			return
		}

		// Handle shared contact
		if update.Message.Contact != nil {
			phone = NormalizePhone(update.Message.Contact.PhoneNumber)
			if phone != "" {
				onPhone(phone, chatID)
				_ = b.SendMessage(chatID, "✅ Номер привязан! Теперь вы будете получать коды входа здесь.")
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		_ = b.SendMessage(chatID, "Отправьте /start <номер_телефона> чтобы привязать аккаунт.\nПример: /start +79991234567")
		w.WriteHeader(http.StatusOK)
	}
}
