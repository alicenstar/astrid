package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/alicenstar/astrid/internal/auth"
	"github.com/alicenstar/astrid/internal/license"
	"github.com/alicenstar/astrid/internal/models"
)

type DashboardHandler struct {
	db             *sql.DB
	rdb            *redis.Client
	tmpl           *Templates
	licenseChecker LicenseChecker
}

func NewDashboardHandler(db *sql.DB, rdb *redis.Client, tmpl *Templates, lc LicenseChecker) *DashboardHandler {
	return &DashboardHandler{db: db, rdb: rdb, tmpl: tmpl, licenseChecker: lc}
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

	// Streak — gated by license entitlement
	var streak int
	streaksEnabled := h.licenseChecker == nil || h.licenseChecker.IsFeatureEnabled("workout_streaks_enabled")
	if streaksEnabled {
		streak, err = models.GetWorkoutStreak(h.db, h.rdb, uid)
		if err != nil {
			h.tmpl.RenderError(w, "Could not load dashboard data", http.StatusInternalServerError)
			return
		}
	}

	// Latest weight and trend
	latestMetric, _ := models.GetLatestBodyMetric(h.db, uid)
	var weightTrend string
	if latestMetric != nil {
		weekAgo, _ := models.GetWeightTrend(h.db, uid, 7)
		if weekAgo != nil {
			diff := latestMetric.WeightKg - *weekAgo
			if diff > 0.1 {
				weightTrend = "up"
			} else if diff < -0.1 {
				weightTrend = "down"
			} else {
				weightTrend = "stable"
			}
		}
	}

	profile, _ := models.GetOrCreateProfile(h.db, uid)
	var bmr, tdee float64
	var weightUnit string
	if profile != nil {
		weightUnit = profile.WeightUnit
		if latestMetric != nil {
			bmr = profile.CalculateBMR(latestMetric.WeightKg)
			tdee = profile.CalculateTDEE(latestMetric.WeightKg)
		}
	}

	ls := license.GetStatus(r)
	data := map[string]any{
		"Title":           "Dashboard",
		"ActiveNav":       "dashboard",
		"Today":           today.Format("Monday, January 2"),
		"TodayDateStr":    today.Format("2006-01-02"),
		"Summary":         summary,
		"HasActivePlan":   activePlan != nil,
		"HasTodayTarget":  summary.CalorieTarget > 0,
		"SplitDay":        splitDay,
		"WorkoutLog":      workoutLog,
		"WorkoutDone":     workoutLog != nil && workoutLog.Completed,
		"Streak":          streak,
		"StreaksEnabled":   streaksEnabled,
		"LatestMetric":    latestMetric,
		"WeightTrend":     weightTrend,
		"WeightUnit":      weightUnit,
		"BMR":             bmr,
		"TDEE":            tdee,
		"LicenseExpired":  ls.Expired,
		"UpdateAvailable": ls.UpdateAvailable,
		"UpdateVersion":   ls.UpdateVersion,
	}
	h.tmpl.Render(w, "dashboard", withUserEmail(r, data))
}
