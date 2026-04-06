package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/alicenstar/astrid/internal/auth"
	"github.com/alicenstar/astrid/internal/models"
)

type DashboardHandler struct {
	db   *sql.DB
	rdb  *redis.Client
	tmpl *Templates
}

func NewDashboardHandler(db *sql.DB, rdb *redis.Client, tmpl *Templates) *DashboardHandler {
	return &DashboardHandler{db: db, rdb: rdb, tmpl: tmpl}
}

func (h *DashboardHandler) Show(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFromContext(r.Context())
	today := time.Now()
	dayOfWeek := int(today.Weekday())

	// Calorie summary
	summary, err := models.GetDailySummary(h.db, h.rdb, uid, today, dayOfWeek)
	if err != nil {
		h.tmpl.RenderError(w, "Could not load dashboard data", http.StatusInternalServerError)
		return
	}

	// Check if user has any active plan (separate from whether today has a target)
	activePlan, err := models.GetActiveCaloriePlan(h.db, uid)
	if err != nil {
		h.tmpl.RenderError(w, "Could not load dashboard data", http.StatusInternalServerError)
		return
	}

	// Today's workout
	splitDay, err := models.GetTodaySplitDay(h.db, uid, dayOfWeek)
	if err != nil {
		h.tmpl.RenderError(w, "Could not load dashboard data", http.StatusInternalServerError)
		return
	}

	// Workout log for today
	workoutLog, err := models.GetWorkoutLog(h.db, uid, today)
	if err != nil {
		h.tmpl.RenderError(w, "Could not load dashboard data", http.StatusInternalServerError)
		return
	}

	// Streak
	streak, err := models.GetWorkoutStreak(h.db, h.rdb, uid)
	if err != nil {
		h.tmpl.RenderError(w, "Could not load dashboard data", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Title":          "Dashboard",
		"ActiveNav":      "dashboard",
		"Today":          today.Format("Monday, January 2"),
		"TodayDateStr":   today.Format("2006-01-02"),
		"Summary":        summary,
		"HasActivePlan":  activePlan != nil,
		"HasTodayTarget": summary.CalorieTarget > 0,
		"SplitDay":       splitDay,
		"WorkoutLog":     workoutLog,
		"WorkoutDone":    workoutLog != nil && workoutLog.Completed,
		"Streak":         streak,
	}
	h.tmpl.Render(w, "dashboard", data)
}
