package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/alicenstar/astrid/internal/auth"
	"github.com/alicenstar/astrid/internal/license"
	"github.com/alicenstar/astrid/internal/models"
)

type ProfileHandler struct {
	db   *sql.DB
	tmpl *Templates
}

func NewProfileHandler(db *sql.DB, tmpl *Templates) *ProfileHandler {
	return &ProfileHandler{db: db, tmpl: tmpl}
}

func (h *ProfileHandler) Page(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFromContext(r.Context())

	profile, err := models.GetOrCreateProfile(h.db, uid)
	if err != nil {
		h.tmpl.RenderError(w, "Could not load profile", http.StatusInternalServerError)
		return
	}

	latest, _ := models.GetLatestBodyMetric(h.db, uid)

	var bmr, tdee float64
	if latest != nil {
		bmr = profile.CalculateBMR(latest.WeightKg)
		tdee = profile.CalculateTDEE(latest.WeightKg)
	}

	ls := license.GetStatus(r)
	data := map[string]any{
		"Title":           "Profile",
		"ActiveNav":       "profile",
		"Profile":         profile,
		"BMR":             bmr,
		"TDEE":            tdee,
		"HasWeight":       latest != nil,
		"LicenseExpired":  ls.Expired,
		"UpdateAvailable": ls.UpdateAvailable,
		"UpdateVersion":   ls.UpdateVersion,
	}
	h.tmpl.Render(w, "profile", withUserEmail(r, data))
}

func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	heightCm, _ := strconv.ParseFloat(r.FormValue("height_cm"), 64)
	birthDate := r.FormValue("birth_date")
	sex := r.FormValue("sex")
	activityLevel := r.FormValue("activity_level")
	weightUnit := r.FormValue("weight_unit")

	if err := models.UpdateProfile(h.db, uid, heightCm, birthDate, sex, activityLevel, weightUnit); err != nil {
		h.tmpl.RenderError(w, "Could not update profile", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}
