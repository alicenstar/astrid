package handlers_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/alicenstar/astrid/internal/auth"
	"github.com/alicenstar/astrid/internal/handlers"
	"github.com/alicenstar/astrid/internal/models"
)

const defaultHandlerTestDSN = "postgres://astrid:astrid@localhost:5432/astrid_test?sslmode=disable"

var (
	handlerDB   *sql.DB
	handlerTmpl *handlers.Templates
	handlerRDB  *redis.Client
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

	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		fmt.Fprintf(os.Stderr, "redis not available: %v\n", err)
		os.Exit(1)
	}

	handlerDB = db
	handlerTmpl = tmpl
	handlerRDB = rdb
	code := m.Run()
	db.Close()
	os.Exit(code)
}

func cleanHandlerDB(t *testing.T, db *sql.DB) {
	t.Helper()
	tables := []string{
		"body_metrics", "user_profiles",
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

func TestProfilePageLoads(t *testing.T) {
	cleanHandlerDB(t, handlerDB)
	r := buildRouter(handlerDB, handlerTmpl)
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Profile") {
		t.Error("expected page to contain 'Profile'")
	}
}

func TestProfileUpdate(t *testing.T) {
	cleanHandlerDB(t, handlerDB)
	r := buildRouter(handlerDB, handlerTmpl)
	form := url.Values{
		"height_cm":      {"175"},
		"birth_date":     {"1990-05-15"},
		"sex":            {"male"},
		"activity_level": {"moderate"},
		"weight_unit":    {"kg"},
	}
	req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", w.Code)
	}
}
func injectUserID(uid uuid.UUID, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := auth.ContextWithUserID(r.Context(), uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func buildRouter(db *sql.DB, tmpl *handlers.Templates) http.Handler {
	user, err := models.EnsureDefaultUser(db)
	if err != nil {
		panic(fmt.Sprintf("ensure default user: %v", err))
	}

	r := chi.NewRouter()

	// Public auth routes (no auth middleware)
	authHandler := handlers.NewAuthHandler(db, handlerRDB, tmpl, "", "", "", nil)
	r.Group(func(r chi.Router) {
		r.Get("/login", authHandler.LoginPage)
		r.Post("/login", authHandler.Login)
		r.Get("/signup", authHandler.SignupPage)
		r.Post("/signup", authHandler.Signup)
		r.Post("/login/demo", authHandler.DemoLogin)
		r.Post("/logout", authHandler.Logout)
	})

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return injectUserID(user.ID, next)
		})

		healthHandler := handlers.NewHealthHandler(
			handlers.NewPgPinger(db),
			handlers.PingerFunc(func() error { return nil }),
		)
		r.Get("/healthz", healthHandler.ServeHTTP)

		plansHandler := handlers.NewPlansHandler(db, nil, tmpl)
		r.Get("/plans", plansHandler.List)
		r.Post("/plans", plansHandler.Create)
		r.Get("/plans/{id}/edit", plansHandler.Edit)
		r.Post("/plans/{id}/edit", plansHandler.Update)
		r.Post("/plans/{id}/activate", plansHandler.Activate)
		r.Post("/plans/{id}/delete", plansHandler.Delete)

		mealsHandler := handlers.NewMealsHandler(db, nil, tmpl)
		r.Get("/log", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/log/"+time.Now().Format("2006-01-02"), http.StatusSeeOther)
		})
		r.Get("/log/{date}", mealsHandler.DailyLog)

		workoutsHandler := handlers.NewWorkoutsHandler(db, tmpl)
		r.Get("/workouts", workoutsHandler.List)
		r.Post("/workouts", workoutsHandler.Create)
		r.Get("/workouts/{id}/edit", workoutsHandler.Edit)
		r.Post("/workouts/{id}/edit", workoutsHandler.Update)
		r.Post("/workouts/{id}/activate", workoutsHandler.Activate)
		r.Post("/workouts/{id}/delete", workoutsHandler.Delete)

		dashboardHandler := handlers.NewDashboardHandler(db, nil, tmpl, nil)
		r.Get("/", dashboardHandler.Show)

		summaryHandler := handlers.NewSummaryHandler(db, nil, tmpl)
		r.Get("/summary", summaryHandler.Show)

		supportHandler := handlers.NewSupportHandler("", "dev", tmpl)
		r.Get("/support", supportHandler.Page)

		profileHandler := handlers.NewProfileHandler(db, tmpl)
		r.Get("/profile", profileHandler.Page)
		r.Post("/profile", profileHandler.Update)
	})

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

func TestDashboardActivePlanNoTargetToday(t *testing.T) {
	cleanHandlerDB(t, handlerDB)

	user, err := models.EnsureDefaultUser(handlerDB)
	if err != nil {
		t.Fatal(err)
	}

	// Create a plan with targets only for days that are NOT today
	today := int(time.Now().Weekday())
	otherDay := (today + 1) % 7
	targets := map[int]int{otherDay: 2000}
	_, err = models.CreateCaloriePlan(handlerDB, user.ID, "Partial Plan", targets)
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
	// Should NOT say "No active calorie plan" — a plan IS active, just no target today
	if strings.Contains(body, "No active calorie plan") {
		t.Fatal("dashboard says 'No active calorie plan' but a plan IS active (just no target for today). Should show different message.")
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

func TestDashboardShowsDailyValuePercent(t *testing.T) {
	cleanHandlerDB(t, handlerDB)

	user, _ := models.EnsureDefaultUser(handlerDB)
	today := time.Now()
	targets := map[int]int{int(today.Weekday()): 2000}
	models.CreateCaloriePlan(handlerDB, user.ID, "Plan", targets)

	// Log a meal with macros
	dl, _ := models.GetOrCreateDailyLog(handlerDB, user.ID, today)
	models.CreateMeal(handlerDB, dl.ID, "Lunch", 500, 25.0, 10.0, 150.0)

	r := buildRouter(handlerDB, handlerTmpl)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	// Should show % DV for protein (25g = 50% of 50g DV)
	if !strings.Contains(body, "% DV") {
		t.Fatal("dashboard should show % daily value for macros")
	}
}

func TestMealLogShowsDailyValuePercent(t *testing.T) {
	cleanHandlerDB(t, handlerDB)

	user, _ := models.EnsureDefaultUser(handlerDB)
	today := time.Now()
	dl, _ := models.GetOrCreateDailyLog(handlerDB, user.ID, today)
	models.CreateMeal(handlerDB, dl.ID, "Dinner", 700, 30.0, 14.0, 200.0)

	r := buildRouter(handlerDB, handlerTmpl)
	req := httptest.NewRequest(http.MethodGet, "/log/"+today.Format("2006-01-02"), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	// Summary section should show % DV
	if !strings.Contains(body, "% DV") {
		t.Fatal("meal log should show % daily value for macro summaries")
	}
	// Individual meal rows should show % DV
	if !strings.Contains(body, "60%") {
		// 30g protein = 60% of 50g DV
		t.Fatal("meal log should show per-meal % daily value")
	}
}

func TestEditPlanPage(t *testing.T) {
	cleanHandlerDB(t, handlerDB)

	user, _ := models.EnsureDefaultUser(handlerDB)
	targets := map[int]int{1: 2000, 2: 2200}
	plan, err := models.CreateCaloriePlan(handlerDB, user.ID, "My Plan", targets)
	if err != nil {
		t.Fatal(err)
	}

	r := buildRouter(handlerDB, handlerTmpl)
	req := httptest.NewRequest(http.MethodGet, "/plans/"+plan.ID.String()+"/edit", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "My Plan") {
		t.Fatal("edit page should show current plan name")
	}
	if !strings.Contains(body, "2000") {
		t.Fatal("edit page should show current day targets")
	}
}

func TestUpdatePlanViaPost(t *testing.T) {
	cleanHandlerDB(t, handlerDB)

	user, _ := models.EnsureDefaultUser(handlerDB)
	targets := map[int]int{1: 2000}
	plan, _ := models.CreateCaloriePlan(handlerDB, user.ID, "Old Name", targets)

	r := buildRouter(handlerDB, handlerTmpl)
	form := strings.NewReader("name=New+Name&day_1=2500&day_3=1800")
	req := httptest.NewRequest(http.MethodPost, "/plans/"+plan.ID.String()+"/edit", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d; body: %s", w.Code, w.Body.String())
	}

	// Verify the update persisted
	updated, _ := models.GetCaloriePlan(handlerDB, plan.ID, user.ID)
	if updated.Name != "New Name" {
		t.Fatalf("expected 'New Name', got %q", updated.Name)
	}
	dayMap := make(map[int]int)
	for _, d := range updated.Days {
		dayMap[d.DayOfWeek] = d.CalorieTarget
	}
	if dayMap[1] != 2500 {
		t.Fatalf("expected Mon=2500, got %d", dayMap[1])
	}
	if dayMap[3] != 1800 {
		t.Fatalf("expected Wed=1800, got %d", dayMap[3])
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

func TestLoginPageRenders(t *testing.T) {
	r := buildRouter(handlerDB, handlerTmpl)
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Log in to Astrid") {
		t.Fatal("login page should render")
	}
	if !strings.Contains(body, "Demo Login") {
		t.Fatal("login page should have demo login button")
	}
}

func TestSignupPageRenders(t *testing.T) {
	r := buildRouter(handlerDB, handlerTmpl)
	req := httptest.NewRequest(http.MethodGet, "/signup", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Create an Account") {
		t.Fatal("signup page should render")
	}
}

func TestDemoLoginCreatesSession(t *testing.T) {
	cleanHandlerDB(t, handlerDB)
	r := buildRouter(handlerDB, handlerTmpl)

	req := httptest.NewRequest(http.MethodPost, "/login/demo", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}

	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "astrid_session" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("demo login should set astrid_session cookie")
	}
}

func TestSignupLoginFlow(t *testing.T) {
	cleanHandlerDB(t, handlerDB)
	r := buildRouter(handlerDB, handlerTmpl)

	// Signup
	form := strings.NewReader("name=Test+User&email=test%40example.com&password=password123")
	req := httptest.NewRequest(http.MethodPost, "/signup", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("signup: expected 303, got %d; body: %s", w.Code, w.Body.String())
	}

	// Login with same credentials
	form = strings.NewReader("email=test%40example.com&password=password123")
	req = httptest.NewRequest(http.MethodPost, "/login", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("login: expected 303, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestLoginWithWrongPassword(t *testing.T) {
	cleanHandlerDB(t, handlerDB)
	r := buildRouter(handlerDB, handlerTmpl)

	models.CreateUser(handlerDB, "Test", "wrong@example.com", "correctpass")

	form := strings.NewReader("email=wrong%40example.com&password=badpassword")
	req := httptest.NewRequest(http.MethodPost, "/login", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (re-render login), got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Invalid email or password") {
		t.Fatal("should show error message for wrong password")
	}
}

func TestSignupValidation(t *testing.T) {
	r := buildRouter(handlerDB, handlerTmpl)

	form := strings.NewReader("name=Test&email=test%40example.com&password=short")
	req := httptest.NewRequest(http.MethodPost, "/signup", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (re-render), got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "at least 8 characters") {
		t.Fatal("should show password validation error")
	}
}

func TestDashboardShowsHealthStub(t *testing.T) {
	cleanHandlerDB(t, handlerDB)
	r := buildRouter(handlerDB, handlerTmpl)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Coming Soon") {
		t.Fatal("dashboard should show health data 'Coming Soon' stub")
	}
}

func TestSupportRoute(t *testing.T) {
	cleanHandlerDB(t, handlerDB)
	r := buildRouter(handlerDB, handlerTmpl)

	req := httptest.NewRequest(http.MethodGet, "/support", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "Generate Support Bundle") {
		t.Fatal("support page should contain 'Generate Support Bundle' button")
	}
}

func TestSummaryRoute(t *testing.T) {
	cleanHandlerDB(t, handlerDB)
	r := buildRouter(handlerDB, handlerTmpl)

	req := httptest.NewRequest(http.MethodGet, "/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestWorkoutsRoute(t *testing.T) {
	cleanHandlerDB(t, handlerDB)
	r := buildRouter(handlerDB, handlerTmpl)

	req := httptest.NewRequest(http.MethodGet, "/workouts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestAllPagesReturn200(t *testing.T) {
	cleanHandlerDB(t, handlerDB)
	r := buildRouter(handlerDB, handlerTmpl)

	pages := []struct {
		name string
		path string
		code int
	}{
		{"dashboard", "/", http.StatusOK},
		{"login", "/login", http.StatusOK},
		{"signup", "/signup", http.StatusOK},
		{"plans", "/plans", http.StatusOK},
		{"workouts", "/workouts", http.StatusOK},
		{"summary", "/summary", http.StatusOK},
		{"support", "/support", http.StatusOK},
		{"healthz", "/healthz", http.StatusOK},
		{"profile", "/profile", http.StatusOK},
	}

	for _, p := range pages {
		t.Run(p.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, p.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != p.code {
				t.Fatalf("%s: expected %d, got %d; body: %.200s", p.name, p.code, w.Code, w.Body.String())
			}
		})
	}
}

