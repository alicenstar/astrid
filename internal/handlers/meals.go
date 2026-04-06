package handlers

import (
	"database/sql"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/alicenstar/astrid/internal/models"
)

type MealsHandler struct {
	db   *sql.DB
	uid  uuid.UUID
	tmpl *template.Template
}

func NewMealsHandler(db *sql.DB, uid uuid.UUID, tmpl *template.Template) *MealsHandler {
	return &MealsHandler{db: db, uid: uid, tmpl: tmpl}
}

func (h *MealsHandler) DailyLog(w http.ResponseWriter, r *http.Request) {
	dateStr := chi.URLParam(r, "date")
	var date time.Time
	if dateStr == "" {
		date = time.Now()
	} else {
		var err error
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			http.Error(w, "invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
			return
		}
	}

	log, err := models.GetOrCreateDailyLog(h.db, h.uid, date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	summary, err := models.GetDailySummary(h.db, h.uid, date, int(date.Weekday()))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	prevDate := date.AddDate(0, 0, -1).Format("2006-01-02")
	nextDate := date.AddDate(0, 0, 1).Format("2006-01-02")

	data := map[string]any{
		"Title":       "Daily Log",
		"ActiveNav":   "log",
		"Log":         log,
		"Summary":     summary,
		"Date":        date,
		"DateStr":     date.Format("2006-01-02"),
		"DateDisplay": date.Format("Monday, January 2"),
		"PrevDate":    prevDate,
		"NextDate":    nextDate,
	}
	h.tmpl.ExecuteTemplate(w, "layout", data)
}

func (h *MealsHandler) AddMeal(w http.ResponseWriter, r *http.Request) {
	dateStr := chi.URLParam(r, "date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log, err := models.GetOrCreateDailyLog(h.db, h.uid, date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	name := r.FormValue("name")
	calories, _ := strconv.Atoi(r.FormValue("calories"))
	protein, _ := strconv.ParseFloat(r.FormValue("protein"), 64)
	fiber, _ := strconv.ParseFloat(r.FormValue("fiber"), 64)
	cholesterol, _ := strconv.ParseFloat(r.FormValue("cholesterol"), 64)

	_, err = models.CreateMeal(h.db, log.ID, name, calories, protein, fiber, cholesterol)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/log/"+dateStr, http.StatusSeeOther)
}

func (h *MealsHandler) DeleteMeal(w http.ResponseWriter, r *http.Request) {
	dateStr := chi.URLParam(r, "date")
	mealID, err := uuid.Parse(chi.URLParam(r, "mealID"))
	if err != nil {
		http.Error(w, "invalid meal id", http.StatusBadRequest)
		return
	}

	if err := models.DeleteMeal(h.db, mealID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/log/"+dateStr, http.StatusSeeOther)
}
