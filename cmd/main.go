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

	handlers.OTPStore = otp.NewStore()

	tgToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if tgToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}
	handlers.TelegramBot = telegram.NewBot(tgToken)

	r := chi.NewRouter()
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)

	r.Use(cors.Handler(cors.Options{
    AllowedOrigins: []string{"*"},
    AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
    AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
    MaxAge:         300,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Post("/auth/send-code", handlers.AuthSendCode)
	r.Post("/auth/verify-code", handlers.AuthVerifyCode)
	r.Post("/auth/google", handlers.AuthGoogle)

	r.Post("/telegram/webhook", handlers.TelegramBot.WebhookHandler(func(phone string, chatID int64) {
		handlers.PhoneChatIDs.Set(phone, chatID)
	}))

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth)

		r.Get("/me", handlers.GetMe)
		r.Patch("/me", handlers.UpdateMe)
		r.Delete("/me", handlers.DeleteMe)
		r.Get("/users/{username}", handlers.GetUserByUsername)

		r.Get("/meetings", handlers.ListMeetings)
		r.Post("/meetings", handlers.CreateMeeting)
		r.Get("/meetings/{id}", handlers.GetMeeting)
		r.Put("/meetings/{id}", handlers.UpdateMeeting)
		r.Delete("/meetings/{id}", handlers.DeleteMeeting)
		r.Post("/meetings/{id}/join", handlers.JoinMeeting)
		r.Delete("/meetings/{id}/join", handlers.LeaveMeeting)

		r.Post("/push/subscribe", handlers.PushSubscribe)
		r.Delete("/push/subscribe", handlers.PushUnsubscribe)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
