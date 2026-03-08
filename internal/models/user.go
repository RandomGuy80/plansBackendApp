package models

import "time"

type User struct {
	ID             string    `db:"id"               json:"id"`
	Phone          string    `db:"phone"            json:"phone"`
	Username       *string   `db:"username"         json:"username"`
	TelegramChatID *int64    `db:"telegram_chat_id" json:"telegram_chat_id,omitempty"`
	CreatedAt      time.Time `db:"created_at"       json:"created_at"`
}
