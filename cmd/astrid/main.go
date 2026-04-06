package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/alicenstar/astrid/internal/config"
	"github.com/alicenstar/astrid/internal/database"
	"github.com/alicenstar/astrid/internal/handlers"
	"github.com/alicenstar/astrid/internal/models"
)

func main() {
	cfg := config.Load()

	db, err := database.ConnectPostgres(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer db.Close()

	if err := database.RunMigrations(db, "migrations"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	rdb, err := database.ConnectRedis(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to connect to redis: %v", err)
	}
	defer rdb.Close()

	user, err := models.EnsureDefaultUser(db)
	if err != nil {
		log.Fatalf("Failed to ensure default user: %v", err)
	}
	log.Printf("Running as user: %s (%s)", user.Name, user.ID)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	healthHandler := handlers.NewHealthHandler(
		handlers.NewPgPinger(db),
		handlers.PingerFunc(func() error {
			return rdb.Ping(context.Background()).Err()
		}),
	)
	r.Get("/healthz", healthHandler.ServeHTTP)

	tmpl, err := handlers.LoadTemplates("internal/templates")
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	plansHandler := handlers.NewPlansHandler(db, user.ID, tmpl)
	r.Get("/plans", plansHandler.List)
	r.Post("/plans", plansHandler.Create)
	r.Post("/plans/{id}/activate", plansHandler.Activate)
	r.Post("/plans/{id}/delete", plansHandler.Delete)

	mealsHandler := handlers.NewMealsHandler(db, rdb, user.ID, tmpl)
	r.Get("/log", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/log/"+time.Now().Format("2006-01-02"), http.StatusSeeOther)
	})
	r.Get("/log/{date}", mealsHandler.DailyLog)
	r.Post("/log/{date}/meals", mealsHandler.AddMeal)
	r.Post("/log/{date}/meals/{mealID}/delete", mealsHandler.DeleteMeal)

	workoutsHandler := handlers.NewWorkoutsHandler(db, user.ID, tmpl)
	r.Get("/workouts", workoutsHandler.List)
	r.Post("/workouts", workoutsHandler.Create)
	r.Post("/workouts/{id}/activate", workoutsHandler.Activate)
	r.Post("/workouts/{id}/delete", workoutsHandler.Delete)
	r.Post("/workouts/days/{dayID}/exercises", workoutsHandler.AddExercise)
	r.Post("/workouts/exercises/{exerciseID}/delete", workoutsHandler.DeleteExercise)

	workoutLogsHandler := handlers.NewWorkoutLogsHandler(db, rdb, user.ID)
	r.Post("/workouts/toggle-today", workoutLogsHandler.Toggle)

	dashboardHandler := handlers.NewDashboardHandler(db, rdb, user.ID, tmpl)
	r.Get("/", dashboardHandler.Show)

	summaryHandler := handlers.NewSummaryHandler(db, rdb, user.ID, tmpl)
	r.Get("/summary", summaryHandler.Show)

	log.Printf("Astrid listening on %s", cfg.Addr())
	if err := http.ListenAndServe(cfg.Addr(), r); err != nil {
		log.Fatal(err)
	}
}
