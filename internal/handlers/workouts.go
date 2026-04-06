package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/alicenstar/astrid/internal/models"
)

type WorkoutsHandler struct {
	db   *sql.DB
	uid  uuid.UUID
	tmpl *Templates
}

func NewWorkoutsHandler(db *sql.DB, uid uuid.UUID, tmpl *Templates) *WorkoutsHandler {
	return &WorkoutsHandler{db: db, uid: uid, tmpl: tmpl}
}

func (h *WorkoutsHandler) List(w http.ResponseWriter, r *http.Request) {
	splits, err := models.ListWorkoutSplits(h.db, h.uid)
	if err != nil {
		h.tmpl.RenderError(w, "Could not load workout splits", http.StatusInternalServerError)
		return
	}
	data := map[string]any{
		"Title":     "Workout Splits",
		"ActiveNav": "workouts",
		"Splits":    splits,
		"DayNames":  models.DayNames,
	}
	h.tmpl.Render(w, "workouts", data)
}

func (h *WorkoutsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	days := make(map[int]string)
	for day := 0; day < 7; day++ {
		label := r.FormValue("day_" + strconv.Itoa(day))
		if label != "" {
			days[day] = label
		}
	}

	_, err := models.CreateWorkoutSplit(h.db, h.uid, name, days)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/workouts", http.StatusSeeOther)
}

func (h *WorkoutsHandler) Activate(w http.ResponseWriter, r *http.Request) {
	splitID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid split id", http.StatusBadRequest)
		return
	}
	if err := models.SetActiveSplit(h.db, h.uid, splitID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/workouts", http.StatusSeeOther)
}

func (h *WorkoutsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	splitID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid split id", http.StatusBadRequest)
		return
	}
	if err := models.DeleteWorkoutSplit(h.db, splitID, h.uid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/workouts", http.StatusSeeOther)
}

func (h *WorkoutsHandler) AddExercise(w http.ResponseWriter, r *http.Request) {
	splitDayID, err := uuid.Parse(chi.URLParam(r, "dayID"))
	if err != nil {
		http.Error(w, "invalid day id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	sets, _ := strconv.Atoi(r.FormValue("sets"))
	reps, _ := strconv.Atoi(r.FormValue("reps"))
	notes := r.FormValue("notes")

	_, err = models.AddExercise(h.db, splitDayID, name, sets, reps, notes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/workouts", http.StatusSeeOther)
}

func (h *WorkoutsHandler) DeleteExercise(w http.ResponseWriter, r *http.Request) {
	exerciseID, err := uuid.Parse(chi.URLParam(r, "exerciseID"))
	if err != nil {
		http.Error(w, "invalid exercise id", http.StatusBadRequest)
		return
	}
	if err := models.DeleteExercise(h.db, exerciseID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/workouts", http.StatusSeeOther)
}
