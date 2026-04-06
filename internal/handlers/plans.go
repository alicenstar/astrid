package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/alicenstar/astrid/internal/auth"
	"github.com/alicenstar/astrid/internal/models"
)

type PlansHandler struct {
	db   *sql.DB
	rdb  *redis.Client
	tmpl *Templates
}

func NewPlansHandler(db *sql.DB, rdb *redis.Client, tmpl *Templates) *PlansHandler {
	return &PlansHandler{db: db, rdb: rdb, tmpl: tmpl}
}

func (h *PlansHandler) invalidateWeekCache(uid uuid.UUID) {
	today := time.Now()
	for i := 0; i < 7; i++ {
		date := today.AddDate(0, 0, -int(today.Weekday())+i)
		models.InvalidateDailyCache(h.rdb, uid, date)
	}
}

func (h *PlansHandler) List(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFromContext(r.Context())
	plans, err := models.ListCaloriePlans(h.db, uid)
	if err != nil {
		h.tmpl.RenderError(w, "Could not load calorie plans", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":     "Calorie Plans",
		"ActiveNav": "plans",
		"Plans":     plans,
		"DayNames":  models.DayNames,
	}
	h.tmpl.Render(w, "plans", data)
}

func (h *PlansHandler) Create(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFromContext(r.Context())
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

	_, err := models.CreateCaloriePlan(h.db, uid, name, targets)
	if err != nil {
		h.tmpl.RenderError(w, "Could not create calorie plan", http.StatusInternalServerError)
		return
	}
	h.invalidateWeekCache(uid)
	http.Redirect(w, r, "/plans", http.StatusSeeOther)
}

func (h *PlansHandler) Activate(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFromContext(r.Context())
	planID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid plan id", http.StatusBadRequest)
		return
	}
	if err := models.SetActivePlan(h.db, uid, planID); err != nil {
		h.tmpl.RenderError(w, "Could not activate plan", http.StatusInternalServerError)
		return
	}
	h.invalidateWeekCache(uid)
	http.Redirect(w, r, "/plans", http.StatusSeeOther)
}

func (h *PlansHandler) Edit(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFromContext(r.Context())
	planID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid plan id", http.StatusBadRequest)
		return
	}
	plan, err := models.GetCaloriePlan(h.db, planID, uid)
	if err != nil || plan == nil {
		h.tmpl.RenderError(w, "Plan not found", http.StatusNotFound)
		return
	}

	dayTargets := make(map[int]int)
	for _, d := range plan.Days {
		dayTargets[d.DayOfWeek] = d.CalorieTarget
	}

	data := map[string]any{
		"Title":      "Edit Plan",
		"ActiveNav":  "plans",
		"Plan":       plan,
		"DayTargets": dayTargets,
		"DayNames":   models.DayNames,
	}
	h.tmpl.Render(w, "plan_edit", data)
}

func (h *PlansHandler) Update(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFromContext(r.Context())
	planID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid plan id", http.StatusBadRequest)
		return
	}
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

	if err := models.UpdateCaloriePlan(h.db, planID, uid, name, targets); err != nil {
		h.tmpl.RenderError(w, "Could not update plan", http.StatusInternalServerError)
		return
	}
	h.invalidateWeekCache(uid)
	http.Redirect(w, r, "/plans", http.StatusSeeOther)
}

func (h *PlansHandler) Delete(w http.ResponseWriter, r *http.Request) {
	uid, _ := auth.UserIDFromContext(r.Context())
	planID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid plan id", http.StatusBadRequest)
		return
	}
	if err := models.DeleteCaloriePlan(h.db, planID, uid); err != nil {
		h.tmpl.RenderError(w, "Could not delete plan", http.StatusInternalServerError)
		return
	}
	h.invalidateWeekCache(uid)
	http.Redirect(w, r, "/plans", http.StatusSeeOther)
}
