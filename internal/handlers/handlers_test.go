package handlers_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"

	"github.com/alicenstar/astrid/internal/handlers"
	"github.com/alicenstar/astrid/internal/models"
)

const defaultHandlerTestDSN = "postgres://astrid:astrid@localhost:5432/astrid_test?sslmode=disable"

var (
	handlerDB   *sql.DB
	handlerTmpl *handlers.Templates
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = defaultHandlerTestDSN
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open test db: %v\n", err)
		os.Exit(1)
	}
	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "ping test db: %v — set TEST_DATABASE_URL or start postgres\n", err)
		os.Exit(1)
	}

	driver, err := migratepostgres.WithInstance(db, &migratepostgres.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate driver: %v\n", err)
		os.Exit(1)
	}
	mg, err := migrate.NewWithDatabaseInstance("file://../../migrations", "postgres", driver)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate init: %v\n", err)
		os.Exit(1)
	}
	if err := mg.Up(); err != nil && err != migrate.ErrNoChange {
		fmt.Fprintf(os.Stderr, "migrate up: %v\n", err)
		os.Exit(1)
	}

	tmpl, err := handlers.LoadTemplates("../../internal/templates")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load templates: %v\n", err)
		os.Exit(1)
	}

	handlerDB = db
	handlerTmpl = tmpl
	code := m.Run()
	db.Close()
	os.Exit(code)
}

func cleanHandlerDB(t *testing.T, db *sql.DB) {
	t.Helper()
	tables := []string{
		"meals", "daily_logs", "workout_logs", "planned_exercises",
		"split_days", "workout_splits", "calorie_plan_days",
		"calorie_plans", "goal_focuses", "users",
	}
	for _, table := range tables {
		_, err := db.Exec("DELETE FROM " + table)
		if err != nil {
			t.Fatalf("clean %s: %v", table, err)
		}
	}
}

func buildRouter(db *sql.DB, tmpl *handlers.Templates) http.Handler {
	user, err := models.EnsureDefaultUser(db)
	if err != nil {
		panic(fmt.Sprintf("ensure default user: %v", err))
	}

	r := chi.NewRouter()

	healthHandler := handlers.NewHealthHandler(
		handlers.NewPgPinger(db),
		handlers.PingerFunc(func() error { return nil }),
	)
	r.Get("/healthz", healthHandler.ServeHTTP)

	plansHandler := handlers.NewPlansHandler(db, nil, user.ID, tmpl)
	r.Get("/plans", plansHandler.List)

	mealsHandler := handlers.NewMealsHandler(db, nil, user.ID, tmpl)
	r.Get("/log", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/log/"+time.Now().Format("2006-01-02"), http.StatusSeeOther)
	})
	r.Get("/log/{date}", mealsHandler.DailyLog)

	dashboardHandler := handlers.NewDashboardHandler(db, nil, user.ID, tmpl)
	r.Get("/", dashboardHandler.Show)

	return r
}

func TestDashboardRoute(t *testing.T) {
	cleanHandlerDB(t, handlerDB)
	r := buildRouter(handlerDB, handlerTmpl)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestPlansRoute(t *testing.T) {
	cleanHandlerDB(t, handlerDB)
	r := buildRouter(handlerDB, handlerTmpl)

	req := httptest.NewRequest(http.MethodGet, "/plans", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestLogRedirect(t *testing.T) {
	cleanHandlerDB(t, handlerDB)
	r := buildRouter(handlerDB, handlerTmpl)

	req := httptest.NewRequest(http.MethodGet, "/log", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}

	location := w.Header().Get("Location")
	today := time.Now().Format("2006-01-02")
	expected := "/log/" + today
	if !strings.HasPrefix(location, expected) {
		t.Fatalf("expected redirect to %s, got %s", expected, location)
	}
}

func TestDashboardShowsActivePlan(t *testing.T) {
	cleanHandlerDB(t, handlerDB)

	// Create user and active plan with targets
	user, err := models.EnsureDefaultUser(handlerDB)
	if err != nil {
		t.Fatal(err)
	}
	targets := map[int]int{0: 2000, 1: 2200, 2: 2200, 3: 2000, 4: 2200, 5: 2200, 6: 1800}
	_, err = models.CreateCaloriePlan(handlerDB, user.ID, "Test Plan", targets)
	if err != nil {
		t.Fatal(err)
	}

	r := buildRouter(handlerDB, handlerTmpl)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// Should NOT contain "No active calorie plan"
	if strings.Contains(body, "No active calorie plan") {
		t.Fatal("dashboard shows 'No active calorie plan' but an active plan with targets exists")
	}
	// Should contain "kcal" (from the progress bar)
	if !strings.Contains(body, "kcal") {
		t.Fatal("dashboard should show calorie target in kcal")
	}
}

func TestDashboardNoActivePlan(t *testing.T) {
	cleanHandlerDB(t, handlerDB)

	r := buildRouter(handlerDB, handlerTmpl)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "No active calorie plan") {
		t.Fatal("dashboard should show 'No active calorie plan' when none exists")
	}
}

func TestLogShowsCalorieProgress(t *testing.T) {
	cleanHandlerDB(t, handlerDB)

	user, err := models.EnsureDefaultUser(handlerDB)
	if err != nil {
		t.Fatal(err)
	}
	targets := map[int]int{0: 2000, 1: 2200, 2: 2200, 3: 2000, 4: 2200, 5: 2200, 6: 1800}
	_, err = models.CreateCaloriePlan(handlerDB, user.ID, "Test Plan", targets)
	if err != nil {
		t.Fatal(err)
	}

	r := buildRouter(handlerDB, handlerTmpl)
	today := time.Now().Format("2006-01-02")
	req := httptest.NewRequest(http.MethodGet, "/log/"+today, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if strings.Contains(body, "No active calorie plan") {
		t.Fatal("log page shows 'No active calorie plan' but an active plan exists")
	}
}

func TestDarkModeToggleExists(t *testing.T) {
	cleanHandlerDB(t, handlerDB)
	r := buildRouter(handlerDB, handlerTmpl)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "theme-toggle") {
		t.Fatal("page should contain theme-toggle button")
	}
	if !strings.Contains(body, `data-theme="dark"`) {
		t.Fatal("page should default to dark theme")
	}
}

func TestHealthzRoute(t *testing.T) {
	r := buildRouter(handlerDB, handlerTmpl)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected JSON content-type, got %s", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"status"`) {
		t.Fatalf("expected JSON with status field, got: %s", body)
	}
}

