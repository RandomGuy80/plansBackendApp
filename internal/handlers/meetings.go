package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"plans-backend/internal/db"
	"plans-backend/internal/middleware"
)

type Meeting struct {
	ID        string    `db:"id"         json:"id"`
	HostID    string    `db:"host_id"    json:"host_id"`
	HostName  *string   `db:"host_name"  json:"host_name"`
	Title     string    `db:"title"      json:"title"`
	Category  string    `db:"category"   json:"category"`
	Place     string    `db:"place"      json:"place"`
	Lat       float64   `db:"lat"        json:"lat"`
	Lng       float64   `db:"lng"        json:"lng"`
	WhenTs    time.Time `db:"when_ts"    json:"when_ts"`
	MaxPeople int       `db:"max_people" json:"max_people"`
	Joined    int       `db:"joined"     json:"joined"`
	Desc      string    `db:"desc"       json:"desc"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// GET /meetings
func ListMeetings(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Queryx(`
		SELECT m.id, m.host_id, u.username as host_name, m.title, m.category,
		       m.place, m.lat, m.lng, m.when_ts, m.max_people, m.joined, m.desc, m.created_at
		FROM meetings m
		LEFT JOIN users u ON u.id = m.host_id
		WHERE m.when_ts > NOW() - INTERVAL '2 hours'
		ORDER BY m.when_ts ASC
	`)
	if err != nil {
		log.Printf("[meetings] list: %v", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	meetings := []Meeting{}
	for rows.Next() {
		var m Meeting
		if err := rows.StructScan(&m); err != nil {
			log.Printf("[meetings] scan: %v", err)
			continue
		}
		meetings = append(meetings, m)
	}
	jsonOK(w, meetings)
}

// POST /meetings
func CreateMeeting(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)

	var body struct {
		Title     string    `json:"title"`
		Category  string    `json:"category"`
		Place     string    `json:"place"`
		Lat       float64   `json:"lat"`
		Lng       float64   `json:"lng"`
		WhenTs    time.Time `json:"when_ts"`
		MaxPeople int       `json:"max_people"`
		Desc      string    `json:"desc"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.Title == "" || body.Place == "" {
		jsonErr(w, "title and place are required", http.StatusBadRequest)
		return
	}
	if body.MaxPeople < 2 {
		body.MaxPeople = 3
	}

	var m Meeting
	err := db.DB.QueryRowx(`
		INSERT INTO meetings (host_id, title, category, place, lat, lng, when_ts, max_people, desc)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, host_id, title, category, place, lat, lng, when_ts, max_people, joined, desc, created_at
	`, userID, body.Title, body.Category, body.Place, body.Lat, body.Lng,
		body.WhenTs, body.MaxPeople, body.Desc).StructScan(&m)
	if err != nil {
		log.Printf("[meetings] create: %v", err)
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Add host as member
	db.DB.Exec(`INSERT INTO meeting_members (meeting_id, user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, m.ID, userID)

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, m)
}

// GET /meetings/{id}
func GetMeeting(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var m Meeting
	err := db.DB.QueryRowx(`
		SELECT m.id, m.host_id, u.username as host_name, m.title, m.category,
		       m.place, m.lat, m.lng, m.when_ts, m.max_people, m.joined, m.desc, m.created_at
		FROM meetings m
		LEFT JOIN users u ON u.id = m.host_id
		WHERE m.id = $1
	`, id).StructScan(&m)
	if err != nil {
		jsonErr(w, "meeting not found", http.StatusNotFound)
		return
	}
	jsonOK(w, m)
}

// PUT /meetings/{id}
func UpdateMeeting(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	id := chi.URLParam(r, "id")

	var body struct {
		Title     *string    `json:"title"`
		Category  *string    `json:"category"`
		Place     *string    `json:"place"`
		Lat       *float64   `json:"lat"`
		Lng       *float64   `json:"lng"`
		WhenTs    *time.Time `json:"when_ts"`
		MaxPeople *int       `json:"max_people"`
		Desc      *string    `json:"desc"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "invalid body", http.StatusBadRequest)
		return
	}

	var m Meeting
	err := db.DB.QueryRowx(`
		UPDATE meetings SET
			title      = COALESCE($1, title),
			category   = COALESCE($2, category),
			place      = COALESCE($3, place),
			lat        = COALESCE($4, lat),
			lng        = COALESCE($5, lng),
			when_ts    = COALESCE($6, when_ts),
			max_people = COALESCE($7, max_people),
			desc       = COALESCE($8, desc)
		WHERE id = $9 AND host_id = $10
		RETURNING id, host_id, title, category, place, lat, lng, when_ts, max_people, joined, desc, created_at
	`, body.Title, body.Category, body.Place, body.Lat, body.Lng,
		body.WhenTs, body.MaxPeople, body.Desc, id, userID).StructScan(&m)
	if err != nil {
		jsonErr(w, "meeting not found or not your meeting", http.StatusNotFound)
		return
	}
	jsonOK(w, m)
}

// DELETE /meetings/{id}
func DeleteMeeting(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	id := chi.URLParam(r, "id")

	res, err := db.DB.Exec(`DELETE FROM meetings WHERE id = $1 AND host_id = $2`, id, userID)
	if err != nil {
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		jsonErr(w, "meeting not found or not your meeting", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /meetings/{id}/join
func JoinMeeting(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	id := chi.URLParam(r, "id")

	tx, err := db.DB.Beginx()
	if err != nil {
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var maxPeople, joined int
	err = tx.QueryRow(`SELECT max_people, joined FROM meetings WHERE id = $1 FOR UPDATE`, id).Scan(&maxPeople, &joined)
	if err != nil {
		jsonErr(w, "meeting not found", http.StatusNotFound)
		return
	}
	if joined >= maxPeople {
		jsonErr(w, "meeting is full", http.StatusConflict)
		return
	}

	_, err = tx.Exec(`INSERT INTO meeting_members (meeting_id, user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, id, userID)
	if err != nil {
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var m Meeting
	err = tx.QueryRowx(`
		UPDATE meetings SET joined = joined + 1 WHERE id = $1
		RETURNING id, host_id, title, category, place, lat, lng, when_ts, max_people, joined, desc, created_at
	`, id).StructScan(&m)
	if err != nil {
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}

	tx.Commit()
	jsonOK(w, m)
}

// DELETE /meetings/{id}/join
func LeaveMeeting(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(string)
	id := chi.URLParam(r, "id")

	res, err := db.DB.Exec(`DELETE FROM meeting_members WHERE meeting_id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		jsonErr(w, "internal server error", http.StatusInternalServerError)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	db.DB.Exec(`UPDATE meetings SET joined = GREATEST(joined - 1, 0) WHERE id = $1`, id)
	w.WriteHeader(http.StatusNoContent)
}
