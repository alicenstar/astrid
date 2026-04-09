package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/alicenstar/astrid/internal/auth"
	"github.com/alicenstar/astrid/internal/license"
	"github.com/alicenstar/astrid/internal/models"
)

type BodyMetricsHandler struct {
	db   *sql.DB
	tmpl *Templates
}

func NewBodyMetricsHandler(db *sql.DB, tmpl *Templates) *BodyMetricsHandler {
	return &BodyMetricsHandler{db: db, tmpl: tmpl}
}

func (h *BodyMetricsHandler) List(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFromContext(r.Context())

	metrics, err := models.ListBodyMetrics(h.db, uid, 30)
	if err != nil {
		h.tmpl.RenderError(w, "Could not load body metrics", http.StatusInternalServerError)
		return
	}

	profile, _ := models.GetOrCreateProfile(h.db, uid)

	ls := license.GetStatus(r)
	data := map[string]any{
		"Title":           "Body Metrics",
		"ActiveNav":       "body_metrics",
		"Metrics":         metrics,
		"WeightUnit":      profile.WeightUnit,
		"TodayDateStr":    time.Now().Format("2006-01-02"),
		"LicenseExpired":  ls.Expired,
		"UpdateAvailable": ls.UpdateAvailable,
		"UpdateVersion":   ls.UpdateVersion,
	}
	h.tmpl.Render(w, "body_metrics", withUserEmail(r, data))
}

func (h *BodyMetricsHandler) Create(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	profile, _ := models.GetOrCreateProfile(h.db, uid)

	weightStr := r.FormValue("weight")
	weight, err := strconv.ParseFloat(weightStr, 64)
	if err != nil || weight <= 0 {
		http.Error(w, "valid weight is required", http.StatusBadRequest)
		return
	}

	// Convert to kg for storage if user prefers lbs
	weightKg := weight
	if profile.WeightUnit == "lbs" {
		weightKg = weight * 0.453592
	}

	var bodyFatPct *float64
	if v := r.FormValue("body_fat_pct"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			bodyFatPct = &f
		}
	}

	var musclePct *float64
	if v := r.FormValue("muscle_pct"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			musclePct = &f
		}
	}

	notes := r.FormValue("notes")
	date := time.Now()
	if d := r.FormValue("date"); d != "" {
		if parsed, err := time.Parse("2006-01-02", d); err == nil {
			date = parsed
		}
	}

	_, err = models.CreateBodyMetric(h.db, uid, date, weightKg, bodyFatPct, musclePct, notes)
	if err != nil {
		h.tmpl.RenderError(w, "Could not save body metric", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/body-metrics", http.StatusSeeOther)
}

func (h *BodyMetricsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFromContext(r.Context())
	metricID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := models.DeleteBodyMetric(h.db, metricID, uid); err != nil {
		h.tmpl.RenderError(w, "Could not delete metric", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/body-metrics", http.StatusSeeOther)
}
