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

type SummaryHandler struct {
	db   *sql.DB
	rdb  *redis.Client
	tmpl *Templates
}

func NewSummaryHandler(db *sql.DB, rdb *redis.Client, tmpl *Templates) *SummaryHandler {
	return &SummaryHandler{db: db, rdb: rdb, tmpl: tmpl}
}

type DaySummaryRow struct {
	Date      string
	DayName   string
	Target    int
	Actual    int
	Adherence int // percentage
}

func (h *SummaryHandler) Show(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFromContext(r.Context())
	today := time.Now()
	// Find the most recent Sunday
	offset := int(today.Weekday())
	weekStart := today.AddDate(0, 0, -offset)

	var rows []DaySummaryRow
	totalTarget := 0
	totalActual := 0

	for i := 0; i < 7; i++ {
		date := weekStart.AddDate(0, 0, i)
		dayOfWeek := int(date.Weekday())

		summary, err := models.GetDailySummary(h.db, h.rdb, uid, date, dayOfWeek)
		if err != nil {
			h.tmpl.RenderError(w, "Could not load weekly summary", http.StatusInternalServerError)
			return
		}

		adherence := 0
		if summary.CalorieTarget > 0 {
			adherence = summary.TotalCalories * 100 / summary.CalorieTarget
		}

		rows = append(rows, DaySummaryRow{
			Date:      date.Format("2006-01-02"),
			DayName:   date.Format("Monday"),
			Target:    summary.CalorieTarget,
			Actual:    summary.TotalCalories,
			Adherence: adherence,
		})

		totalTarget += summary.CalorieTarget
		totalActual += summary.TotalCalories
	}

	weekAdherence := 0
	if totalTarget > 0 {
		weekAdherence = totalActual * 100 / totalTarget
	}

	ls := license.GetStatus(r)
	data := map[string]any{
		"Title":           "Weekly Summary",
		"ActiveNav":       "summary",
		"Days":            rows,
		"TotalTarget":     totalTarget,
		"TotalActual":     totalActual,
		"WeekAdherence":   weekAdherence,
		"WeekOf":          weekStart.Format("January 2"),
		"LicenseExpired":  ls.Expired,
		"UpdateAvailable": ls.UpdateAvailable,
		"UpdateVersion":   ls.UpdateVersion,
	}
	h.tmpl.Render(w, "summary", withUserEmail(r, data))
}
