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
	// ── Init DB ───────────────────────────────────────────────────────────────
	db.Connect()

	// ── Init OTP store ────────────────────────────────────────────────────────
	handlers.OTPStore = otp.NewStore()

	// ── Init Telegram bot ─────────────────────────────────────────────────────
	tgToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if tgToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}
	handlers.TelegramBot = telegram.NewBot(tgToken)

	// ── Router ────────────────────────────────────────────────────────────────
	r := chi.NewRouter()

	// ── Middleware ────────────────────────────────────────────────────────────
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)

	// ── CORS ──────────────────────────────────────────────────────────────────
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{
			"https://golden-bavarois-28286a.netlify.app",
			"http://localhost:3000",
			"http://localhost:8080",
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// ── Public routes ─────────────────────────────────────────────────────────
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Post("/auth/send-code", handlers.AuthSendCode)
	r.Post("/auth/verify-code", handlers.AuthVerifyCode)
	r.Post("/auth/google", handlers.AuthGoogle)

	// Telegram webhook
	r.Post("/telegram/webhook", handlers.TelegramBot.WebhookHandler(func(phone string, chatID int64) {
		handlers.PhoneChatIDs.Set(phone, chatID)
	}))

	// ── Protected routes ──────────────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth)

		r.Get("/me", handlers.GetMe)
		r.Patch("/me", handlers.UpdateMe)
		r.Delete("/me", handlers.DeleteMe)
		r.Get("/users/{username}", handlers.GetUserByUsername)

		r.Post("/push/subscribe", handlers.PushSubscribe)
		r.Delete("/push/subscribe", handlers.PushUnsubscribe)
	})

	// ── Start server ──────────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
