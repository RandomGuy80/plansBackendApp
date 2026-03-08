package handlers

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"

	webpush "github.com/SherClockHolmes/webpush-go"

	"plans-backend/internal/db"
	"plans-backend/internal/middleware"
)

// ── Types ─────────────────────────────────────────────────────────────────────

type PushSubscription struct {
	Endpoint string `json:"endpoint" db:"endpoint"`
	P256dh   string `json:"p256dh"   db:"p256dh"`
	Auth     string `json:"auth"     db:"auth"`
}

type pushSubReq struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

type PushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Tag   string `json:"tag"`
	URL   string `json:"url"`
}

// ── POST /push/subscribe ──────────────────────────────────────────────────────

func PushSubscribe(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	var req pushSubReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.Endpoint == "" || req.Keys.P256dh == "" || req.Keys.Auth == "" {
		jsonErr(w, "missing subscription fields", http.StatusBadRequest)
		return
	}

	_, err := db.DB.Exec(`
		INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (endpoint) DO UPDATE
		  SET user_id = EXCLUDED.user_id,
		      p256dh  = EXCLUDED.p256dh,
		      auth    = EXCLUDED.auth
	`, userID, req.Endpoint, req.Keys.P256dh, req.Keys.Auth)
	if err != nil {
		log.Printf("[push] subscribe: %v", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("[push] subscribed user=%s", userID)
	jsonOK(w, map[string]string{"status": "subscribed"})
}

// ── DELETE /push/subscribe ────────────────────────────────────────────────────

func PushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	db.DB.Exec(`DELETE FROM push_subscriptions WHERE user_id = $1`, userID)
	w.WriteHeader(http.StatusNoContent)
}

// ── SendPushToUser ────────────────────────────────────────────────────────────

func SendPushToUser(userID string, payload PushPayload) {
	rows, err := db.DB.Queryx(`
		SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = $1
	`, userID)
	if err != nil {
		log.Printf("[push] query subs: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var sub PushSubscription
		if err := rows.StructScan(&sub); err != nil {
			continue
		}
		go sendWebPush(sub, payload)
	}
}

func sendWebPush(sub PushSubscription, payload PushPayload) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	s := &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256dh,
			Auth:   sub.Auth,
		},
	}

	resp, err := webpush.SendNotification(body, s, &webpush.Options{
		TTL:             86400,
		Urgency:         webpush.UrgencyNormal,
		VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
		Subscriber:      "mailto:gridin722@gmail.com",
	})
	if err != nil {
		log.Printf("[push] send error: %v", err)
		return
	}
	defer resp.Body.Close()

	// 410 Gone / 404 — подписка протухла, удаляем
	if resp.StatusCode == 410 || resp.StatusCode == 404 {
		db.DB.Exec(`DELETE FROM push_subscriptions WHERE endpoint = $1`, sub.Endpoint)
		log.Printf("[push] removed expired subscription")
	}
}

// ── GenerateVAPIDKeys ─────────────────────────────────────────────────────────

func GenerateVAPIDKeys() (pub, priv string, err error) {
	curve := ecdh.P256()
	key, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return
	}
	priv = base64.RawURLEncoding.EncodeToString(key.Bytes())
	pub = base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())
	return
}
