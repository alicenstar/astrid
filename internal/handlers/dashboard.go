package handlers

import (
	"database/sql"
	"html/template"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/alicenstar/astrid/internal/models"
)

type DashboardHandler struct {
	db   *sql.DB
	rdb  *redis.Client
	uid  uuid.UUID
	tmpl *template.Template
}

func NewDashboardHandler(db *sql.DB, rdb *redis.Client, uid uuid.UUID, tmpl *template.Template) *DashboardHandler {
	return &DashboardHandler{db: db, rdb: rdb, uid: uid, tmpl: tmpl}
}

func (h *DashboardHandler) Show(w http.ResponseWriter, r *http.Request) {
	today := time.Now()
	dayOfWeek := int(today.Weekday())

	// Calorie summary
	summary, err := models.GetDailySummary(h.db, h.rdb, h.uid, today, dayOfWeek)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Today's workout
	splitDay, err := models.GetTodaySplitDay(h.db, h.uid, dayOfWeek)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Workout log for today
	workoutLog, err := models.GetWorkoutLog(h.db, h.uid, today)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Streak
	streak, err := models.GetWorkoutStreak(h.db, h.rdb, h.uid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Title":        "Dashboard",
		"ActiveNav":    "dashboard",
		"Today":        today.Format("Monday, January 2"),
		"TodayDateStr": today.Format("2006-01-02"),
		"Summary":      summary,
		"SplitDay":     splitDay,
		"WorkoutLog":   workoutLog,
		"WorkoutDone":  workoutLog != nil && workoutLog.Completed,
		"Streak":       streak,
	}
	h.tmpl.ExecuteTemplate(w, "layout", data)
}
