package handlers

import (
	"database/sql"
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/alicenstar/astrid/internal/models"
)

type PlansHandler struct {
	db   *sql.DB
	uid  uuid.UUID
	tmpl *template.Template
}

func NewPlansHandler(db *sql.DB, uid uuid.UUID, tmpl *template.Template) *PlansHandler {
	return &PlansHandler{db: db, uid: uid, tmpl: tmpl}
}

func (h *PlansHandler) List(w http.ResponseWriter, r *http.Request) {
	plans, err := models.ListCaloriePlans(h.db, h.uid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":     "Calorie Plans",
		"ActiveNav": "plans",
		"Plans":     plans,
		"DayNames":  models.DayNames,
	}
	h.tmpl.ExecuteTemplate(w, "layout", data)
}

func (h *PlansHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	targets := make(map[int]int)
	for day := 0; day < 7; day++ {
		val := r.FormValue("day_" + strconv.Itoa(day))
		if val != "" {
			target, err := strconv.Atoi(val)
			if err == nil && target > 0 {
				targets[day] = target
			}
		}
	}

	_, err := models.CreateCaloriePlan(h.db, h.uid, name, targets)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/plans", http.StatusSeeOther)
}

func (h *PlansHandler) Activate(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid plan id", http.StatusBadRequest)
		return
	}
	if err := models.SetActivePlan(h.db, h.uid, planID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/plans", http.StatusSeeOther)
}

func (h *PlansHandler) Delete(w http.ResponseWriter, r *http.Request) {
	planID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid plan id", http.StatusBadRequest)
		return
	}
	if err := models.DeleteCaloriePlan(h.db, planID, h.uid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/plans", http.StatusSeeOther)
}
