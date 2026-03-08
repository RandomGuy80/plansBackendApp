package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"plans-backend/internal/db"
	"plans-backend/internal/handlers"
	"plans-backend/internal/middleware"
	"plans-backend/internal/otp"
	"plans-backend/internal/telegram"
)

func main() {
	db.Connect()

	// ── Telegram bot ──────────────────────────────────────────────────────────
	tgToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if tgToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN env variable is required")
	}
	bot := telegram.NewBot(tgToken)
	handlers.TelegramBot = bot
	handlers.OTPStore = otp.NewStore()

	// ── Router ────────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
	}))

	// ── Public ────────────────────────────────────────────────────────────────
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Telegram webhook — вызывается Telegram когда пользователь пишет боту.
	// Зарегистрируй webhook один раз:
	// GET https://api.telegram.org/bot<TOKEN>/setWebhook?url=https://yourdomain/tg/webhook
	r.Post("/tg/webhook", bot.WebhookHandler(func(phone string, chatID int64) {
		handlers.PhoneChatIDs.Set(phone, chatID)
		log.Printf("[tg] linked phone=%s chat_id=%d", phone, chatID)
	}))

	// Phone auth — двухшаговая
	r.Post("/auth/phone/send", handlers.AuthPhoneSend)     // 1. отправить OTP
	r.Post("/auth/phone/verify", handlers.AuthPhoneVerify) // 2. проверить OTP → JWT

	r.Post("/auth/google", handlers.AuthGoogle)

	// ── Protected ─────────────────────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth)

		r.Get("/me", handlers.GetMe)
		r.Patch("/me", handlers.UpdateMe)
		r.Get("/users/{username}", handlers.GetUserByUsername)

		r.Get("/meetings/my", handlers.GetMyMeetings)
		r.Get("/meetings", handlers.GetMeetings)
		r.Post("/meetings", handlers.CreateMeeting)
		r.Delete("/meetings/{id}", handlers.DeleteMeeting)
		r.Post("/meetings/{id}/join", handlers.JoinMeeting)
		r.Delete("/meetings/{id}/join", handlers.LeaveMeeting)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server running on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
