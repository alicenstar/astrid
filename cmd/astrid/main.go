package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	authpkg "github.com/alicenstar/astrid/internal/auth"
	"github.com/alicenstar/astrid/internal/config"
	"github.com/alicenstar/astrid/internal/database"
	"github.com/alicenstar/astrid/internal/handlers"
	"github.com/alicenstar/astrid/internal/license"
	"github.com/alicenstar/astrid/internal/metrics"
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

	tmpl, err := handlers.LoadTemplates("internal/templates")
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	var licenseChecker handlers.LicenseChecker
	var licenseClient *license.Client
	if cfg.ReplicatedSDKURL != "" {
		licenseClient = license.NewClient(cfg.ReplicatedSDKURL)
		licenseChecker = licenseClient
		reporter := metrics.NewReplicatedReporter(cfg.ReplicatedSDKURL)
		go func() {
			ticker := time.NewTicker(4 * time.Hour)
			defer ticker.Stop()
			for {
				m, err := models.GetAppMetrics(db)
				if err != nil {
					log.Printf("WARN: failed to gather app metrics: %v", err)
				} else {
					reporter.ReportAppMetrics(m.UserCount, m.PlanCount, m.MealCount, m.WorkoutCount)
				}
				<-ticker.C
			}
		}()
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Public routes
	healthHandler := handlers.NewHealthHandler(
		handlers.NewPgPinger(db),
		handlers.PingerFunc(func() error {
			return rdb.Ping(context.Background()).Err()
		}),
	)
	r.Get("/healthz", healthHandler.ServeHTTP)

	authHandler := handlers.NewAuthHandler(db, rdb, tmpl, cfg.GoogleClientID, cfg.GoogleSecret, cfg.GoogleRedirectURL, licenseChecker)
	r.Get("/login", authHandler.LoginPage)
	r.Post("/login", authHandler.Login)
	r.Get("/signup", authHandler.SignupPage)
	r.Post("/signup", authHandler.Signup)
	r.Post("/login/demo", authHandler.DemoLogin)
	r.Post("/logout", authHandler.Logout)
	r.Get("/auth/google", authHandler.GoogleLogin)
	r.Get("/auth/google/callback", authHandler.GoogleCallback)

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(authpkg.NewAuthMiddleware(rdb))
		if licenseClient != nil {
			r.Use(license.StatusMiddleware(licenseClient))
		}

		dashboardHandler := handlers.NewDashboardHandler(db, rdb, tmpl, licenseChecker)
		r.Get("/", dashboardHandler.Show)

		plansHandler := handlers.NewPlansHandler(db, rdb, tmpl)
		r.Get("/plans", plansHandler.List)
		r.Post("/plans", plansHandler.Create)
		r.Get("/plans/{id}/edit", plansHandler.Edit)
		r.Post("/plans/{id}/edit", plansHandler.Update)
		r.Post("/plans/{id}/activate", plansHandler.Activate)
		r.Post("/plans/{id}/delete", plansHandler.Delete)

		mealsHandler := handlers.NewMealsHandler(db, rdb, tmpl)
		r.Get("/log", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/log/"+time.Now().Format("2006-01-02"), http.StatusSeeOther)
		})
		r.Get("/log/{date}", mealsHandler.DailyLog)
		r.Post("/log/{date}/meals", mealsHandler.AddMeal)
		r.Post("/log/{date}/meals/{mealID}/delete", mealsHandler.DeleteMeal)

		workoutsHandler := handlers.NewWorkoutsHandler(db, tmpl)
		r.Get("/workouts", workoutsHandler.List)
		r.Post("/workouts", workoutsHandler.Create)
		r.Get("/workouts/{id}/edit", workoutsHandler.Edit)
		r.Post("/workouts/{id}/edit", workoutsHandler.Update)
		r.Post("/workouts/{id}/activate", workoutsHandler.Activate)
		r.Post("/workouts/{id}/delete", workoutsHandler.Delete)
		r.Post("/workouts/days/{dayID}/exercises", workoutsHandler.AddExercise)
		r.Post("/workouts/exercises/{exerciseID}/delete", workoutsHandler.DeleteExercise)

		workoutLogsHandler := handlers.NewWorkoutLogsHandler(db, rdb)
		r.Post("/workouts/toggle-today", workoutLogsHandler.Toggle)

		summaryHandler := handlers.NewSummaryHandler(db, rdb, tmpl)
		r.Get("/summary", summaryHandler.Show)
	})

	log.Printf("Astrid listening on %s", cfg.Addr())
	if err := http.ListenAndServe(cfg.Addr(), r); err != nil {
		log.Fatal(err)
	}
}
