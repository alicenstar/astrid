package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/alicenstar/astrid/internal/auth"
	"github.com/alicenstar/astrid/internal/models"
)

type WorkoutLogsHandler struct {
	db  *sql.DB
	rdb *redis.Client
}

func NewWorkoutLogsHandler(db *sql.DB, rdb *redis.Client) *WorkoutLogsHandler {
	return &WorkoutLogsHandler{db: db, rdb: rdb}
}

func (h *WorkoutLogsHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFromContext(r.Context())
	today := time.Now()
	splitDay, _ := models.GetTodaySplitDay(h.db, uid, int(today.Weekday()))

	var splitDayID *uuid.UUID
	if splitDay != nil {
		splitDayID = &splitDay.ID
	}

	if err := models.ToggleWorkoutComplete(h.db, h.rdb, uid, today, splitDayID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
