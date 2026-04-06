package models

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

const defaultTestDSN = "postgres://astrid:astrid@localhost:5432/astrid_test?sslmode=disable"

var testDB *sql.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = defaultTestDSN
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

	// Run migrations
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

	testDB = db
	code := m.Run()
	db.Close()
	os.Exit(code)
}

func cleanDB(t *testing.T, db *sql.DB) {
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

func TestEnsureDefaultUser(t *testing.T) {
	cleanDB(t, testDB)

	u1, err := EnsureDefaultUser(testDB)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if u1 == nil {
		t.Fatal("expected user, got nil")
	}
	if u1.Email != "default@astrid.fit" {
		t.Fatalf("unexpected email: %s", u1.Email)
	}

	// Second call should return the same user
	u2, err := EnsureDefaultUser(testDB)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if u2.ID != u1.ID {
		t.Fatalf("expected same user id %s, got %s", u1.ID, u2.ID)
	}
}

func TestCreateCaloriePlan_AutoActivates(t *testing.T) {
	cleanDB(t, testDB)

	user, err := EnsureDefaultUser(testDB)
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}

	targets := map[int]int{1: 2000, 2: 2100, 3: 2000}
	plan, err := CreateCaloriePlan(testDB, user.ID, "First Plan", targets)
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	if !plan.IsActive {
		t.Error("first plan should be auto-activated")
	}
}

func TestCreateCaloriePlan_SecondNotAutoActivated(t *testing.T) {
	cleanDB(t, testDB)

	user, err := EnsureDefaultUser(testDB)
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}

	targets := map[int]int{1: 2000}
	_, err = CreateCaloriePlan(testDB, user.ID, "First Plan", targets)
	if err != nil {
		t.Fatalf("create first plan: %v", err)
	}

	second, err := CreateCaloriePlan(testDB, user.ID, "Second Plan", targets)
	if err != nil {
		t.Fatalf("create second plan: %v", err)
	}

	if second.IsActive {
		t.Error("second plan should NOT be auto-activated when first is active")
	}
}

func TestSetActivePlan(t *testing.T) {
	cleanDB(t, testDB)

	user, err := EnsureDefaultUser(testDB)
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}

	targets := map[int]int{1: 2000}
	first, err := CreateCaloriePlan(testDB, user.ID, "First", targets)
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := CreateCaloriePlan(testDB, user.ID, "Second", targets)
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	if err := SetActivePlan(testDB, user.ID, second.ID); err != nil {
		t.Fatalf("set active: %v", err)
	}

	plans, err := ListCaloriePlans(testDB, user.ID)
	if err != nil {
		t.Fatalf("list plans: %v", err)
	}

	for _, p := range plans {
		if p.ID == first.ID && p.IsActive {
			t.Error("first plan should no longer be active")
		}
		if p.ID == second.ID && !p.IsActive {
			t.Error("second plan should now be active")
		}
	}
}

func TestGetTodayCalorieTarget(t *testing.T) {
	cleanDB(t, testDB)

	user, err := EnsureDefaultUser(testDB)
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}

	// Use day 1 (Monday) with target 1800
	targets := map[int]int{1: 1800, 2: 2000}
	_, err = CreateCaloriePlan(testDB, user.ID, "Test Plan", targets)
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	got, err := GetTodayCalorieTarget(testDB, user.ID, 1)
	if err != nil {
		t.Fatalf("get target: %v", err)
	}
	if got != 1800 {
		t.Fatalf("expected 1800, got %d", got)
	}
}

func TestGetTodayCalorieTarget_NoPlan(t *testing.T) {
	cleanDB(t, testDB)

	user, err := EnsureDefaultUser(testDB)
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}

	got, err := GetTodayCalorieTarget(testDB, user.ID, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0 {
		t.Fatalf("expected 0 with no plan, got %d", got)
	}
}

func TestCreateAndListMeals(t *testing.T) {
	cleanDB(t, testDB)

	user, err := EnsureDefaultUser(testDB)
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}

	today := time.Now()
	log, err := GetOrCreateDailyLog(testDB, user.ID, today)
	if err != nil {
		t.Fatalf("get/create daily log: %v", err)
	}

	_, err = CreateMeal(testDB, log.ID, "Oats", 350, 12.0, 4.0, 0.0)
	if err != nil {
		t.Fatalf("create meal 1: %v", err)
	}
	_, err = CreateMeal(testDB, log.ID, "Chicken", 450, 40.0, 0.0, 85.0)
	if err != nil {
		t.Fatalf("create meal 2: %v", err)
	}

	meals, err := ListMeals(testDB, log.ID)
	if err != nil {
		t.Fatalf("list meals: %v", err)
	}
	if len(meals) != 2 {
		t.Fatalf("expected 2 meals, got %d", len(meals))
	}

	total := 0
	for _, m := range meals {
		total += m.Calories
	}
	if total != 800 {
		t.Fatalf("expected total calories 800, got %d", total)
	}
}

func TestDeleteMeal(t *testing.T) {
	cleanDB(t, testDB)

	user, err := EnsureDefaultUser(testDB)
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}

	today := time.Now()
	log, err := GetOrCreateDailyLog(testDB, user.ID, today)
	if err != nil {
		t.Fatalf("get/create daily log: %v", err)
	}

	meal, err := CreateMeal(testDB, log.ID, "Salad", 200, 5.0, 3.0, 0.0)
	if err != nil {
		t.Fatalf("create meal: %v", err)
	}

	if err := DeleteMeal(testDB, meal.ID); err != nil {
		t.Fatalf("delete meal: %v", err)
	}

	meals, err := ListMeals(testDB, log.ID)
	if err != nil {
		t.Fatalf("list meals after delete: %v", err)
	}
	if len(meals) != 0 {
		t.Fatalf("expected 0 meals after delete, got %d", len(meals))
	}
}

func TestGetDailySummary(t *testing.T) {
	cleanDB(t, testDB)

	user, err := EnsureDefaultUser(testDB)
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}

	today := time.Now()
	log, err := GetOrCreateDailyLog(testDB, user.ID, today)
	if err != nil {
		t.Fatalf("get/create daily log: %v", err)
	}

	_, err = CreateMeal(testDB, log.ID, "Breakfast", 500, 20.0, 5.0, 10.0)
	if err != nil {
		t.Fatalf("create meal 1: %v", err)
	}
	_, err = CreateMeal(testDB, log.ID, "Lunch", 700, 35.0, 8.0, 50.0)
	if err != nil {
		t.Fatalf("create meal 2: %v", err)
	}

	dayOfWeek := int(today.Weekday())
	summary, err := GetDailySummary(testDB, nil, user.ID, today, dayOfWeek)
	if err != nil {
		t.Fatalf("get daily summary: %v", err)
	}

	if summary.TotalCalories != 1200 {
		t.Errorf("expected TotalCalories 1200, got %d", summary.TotalCalories)
	}
	if summary.TotalProtein != 55.0 {
		t.Errorf("expected TotalProtein 55.0, got %.1f", summary.TotalProtein)
	}
	if summary.TotalFiber != 13.0 {
		t.Errorf("expected TotalFiber 13.0, got %.1f", summary.TotalFiber)
	}
	if summary.TotalCholesterol != 60.0 {
		t.Errorf("expected TotalCholesterol 60.0, got %.1f", summary.TotalCholesterol)
	}
}

func TestCreateWorkoutSplit(t *testing.T) {
	cleanDB(t, testDB)

	user, err := EnsureDefaultUser(testDB)
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}

	days := map[int]string{
		1: "Push",
		2: "Pull",
		3: "Legs",
	}
	split, err := CreateWorkoutSplit(testDB, user.ID, "PPL", days)
	if err != nil {
		t.Fatalf("create split: %v", err)
	}

	if split.Name != "PPL" {
		t.Errorf("expected name PPL, got %s", split.Name)
	}
	if len(split.Days) != 3 {
		t.Errorf("expected 3 days, got %d", len(split.Days))
	}

	// Verify labels
	labels := map[int]string{}
	for _, d := range split.Days {
		labels[d.DayOfWeek] = d.Label
	}
	for dayNum, expectedLabel := range days {
		if labels[dayNum] != expectedLabel {
			t.Errorf("day %d: expected label %s, got %s", dayNum, expectedLabel, labels[dayNum])
		}
	}
}

func TestToggleWorkoutComplete(t *testing.T) {
	cleanDB(t, testDB)

	user, err := EnsureDefaultUser(testDB)
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}

	today := time.Now()

	// First toggle — should create a completed log
	if err := ToggleWorkoutComplete(testDB, nil, user.ID, today, nil); err != nil {
		t.Fatalf("first toggle: %v", err)
	}

	wl, err := GetWorkoutLog(testDB, user.ID, today)
	if err != nil {
		t.Fatalf("get workout log after first toggle: %v", err)
	}
	if wl == nil {
		t.Fatal("expected workout log, got nil")
	}
	if !wl.Completed {
		t.Error("expected completed=true after first toggle")
	}

	// Second toggle — should flip to not completed
	if err := ToggleWorkoutComplete(testDB, nil, user.ID, today, nil); err != nil {
		t.Fatalf("second toggle: %v", err)
	}

	wl2, err := GetWorkoutLog(testDB, user.ID, today)
	if err != nil {
		t.Fatalf("get workout log after second toggle: %v", err)
	}
	if wl2 == nil {
		t.Fatal("expected workout log, got nil")
	}
	if wl2.Completed {
		t.Error("expected completed=false after second toggle")
	}
}

func TestGetWorkoutStreak(t *testing.T) {
	cleanDB(t, testDB)

	user, err := EnsureDefaultUser(testDB)
	if err != nil {
		t.Fatalf("ensure user: %v", err)
	}

	// Insert 3 consecutive completed days ending today
	today := time.Now().Truncate(24 * time.Hour)
	for i := 0; i < 3; i++ {
		date := today.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		_, err := testDB.Exec(
			`INSERT INTO workout_logs (user_id, date, completed) VALUES ($1, $2, true)`,
			user.ID, dateStr,
		)
		if err != nil {
			t.Fatalf("insert workout log day -%d: %v", i, err)
		}
	}

	streak, err := GetWorkoutStreak(testDB, nil, user.ID)
	if err != nil {
		t.Fatalf("get streak: %v", err)
	}
	if streak != 3 {
		t.Fatalf("expected streak 3, got %d", streak)
	}
}

func TestUpdateCaloriePlan(t *testing.T) {
	cleanDB(t, testDB)
	user, _ := EnsureDefaultUser(testDB)

	// Create a plan with targets for Mon and Tue
	targets := map[int]int{1: 2000, 2: 2200}
	plan, err := CreateCaloriePlan(testDB, user.ID, "Original", targets)
	if err != nil {
		t.Fatal(err)
	}

	// Update: rename and change targets (add Wed, change Mon, remove Tue)
	newTargets := map[int]int{1: 1800, 3: 2500}
	err = UpdateCaloriePlan(testDB, plan.ID, user.ID, "Updated Name", newTargets)
	if err != nil {
		t.Fatal(err)
	}

	// Verify name changed
	plans, _ := ListCaloriePlans(testDB, user.ID)
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
	if plans[0].Name != "Updated Name" {
		t.Fatalf("expected name 'Updated Name', got %q", plans[0].Name)
	}

	// Verify days: should have Mon=1800 and Wed=2500 (Tue removed)
	days := plans[0].Days
	if len(days) != 2 {
		t.Fatalf("expected 2 days, got %d", len(days))
	}

	dayMap := make(map[int]int)
	for _, d := range days {
		dayMap[d.DayOfWeek] = d.CalorieTarget
	}
	if dayMap[1] != 1800 {
		t.Fatalf("expected Mon=1800, got %d", dayMap[1])
	}
	if dayMap[3] != 2500 {
		t.Fatalf("expected Wed=2500, got %d", dayMap[3])
	}
	if _, ok := dayMap[2]; ok {
		t.Fatal("Tue should have been removed")
	}
}

func TestGetCaloriePlan(t *testing.T) {
	cleanDB(t, testDB)
	user, _ := EnsureDefaultUser(testDB)

	targets := map[int]int{0: 2000, 1: 2200}
	created, _ := CreateCaloriePlan(testDB, user.ID, "My Plan", targets)

	plan, err := GetCaloriePlan(testDB, created.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil {
		t.Fatal("expected plan, got nil")
	}
	if plan.Name != "My Plan" {
		t.Fatalf("expected 'My Plan', got %q", plan.Name)
	}
	if len(plan.Days) != 2 {
		t.Fatalf("expected 2 days, got %d", len(plan.Days))
	}
}

func TestCreateWorkoutSplit_AutoActivates(t *testing.T) {
	cleanDB(t, testDB)
	user, _ := EnsureDefaultUser(testDB)

	days := map[int]string{1: "Push", 2: "Pull"}
	split, err := CreateWorkoutSplit(testDB, user.ID, "PPL", days)
	if err != nil {
		t.Fatal(err)
	}
	if !split.IsActive {
		t.Fatal("first workout split should be auto-activated")
	}
}

func TestCreateWorkoutSplit_SecondNotAutoActivated(t *testing.T) {
	cleanDB(t, testDB)
	user, _ := EnsureDefaultUser(testDB)

	days1 := map[int]string{1: "Push"}
	CreateWorkoutSplit(testDB, user.ID, "Split 1", days1)

	days2 := map[int]string{2: "Pull"}
	split2, err := CreateWorkoutSplit(testDB, user.ID, "Split 2", days2)
	if err != nil {
		t.Fatal(err)
	}
	if split2.IsActive {
		t.Fatal("second workout split should NOT be auto-activated")
	}
}

func TestUpdateWorkoutSplit(t *testing.T) {
	cleanDB(t, testDB)
	user, _ := EnsureDefaultUser(testDB)

	days := map[int]string{1: "Push", 2: "Pull"}
	split, _ := CreateWorkoutSplit(testDB, user.ID, "Old PPL", days)

	newDays := map[int]string{1: "Upper", 3: "Lower"}
	err := UpdateWorkoutSplit(testDB, split.ID, user.ID, "New Split", newDays)
	if err != nil {
		t.Fatal(err)
	}

	splits, _ := ListWorkoutSplits(testDB, user.ID)
	if splits[0].Name != "New Split" {
		t.Fatalf("expected 'New Split', got %q", splits[0].Name)
	}
	if len(splits[0].Days) != 2 {
		t.Fatalf("expected 2 days, got %d", len(splits[0].Days))
	}

	dayMap := make(map[int]string)
	for _, d := range splits[0].Days {
		dayMap[d.DayOfWeek] = d.Label
	}
	if dayMap[1] != "Upper" {
		t.Fatalf("expected Mon='Upper', got %q", dayMap[1])
	}
	if dayMap[3] != "Lower" {
		t.Fatalf("expected Wed='Lower', got %q", dayMap[3])
	}
}

func TestGetWorkoutSplit(t *testing.T) {
	cleanDB(t, testDB)
	user, _ := EnsureDefaultUser(testDB)

	days := map[int]string{1: "Push", 2: "Pull"}
	created, _ := CreateWorkoutSplit(testDB, user.ID, "PPL", days)

	split, err := GetWorkoutSplit(testDB, created.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if split == nil {
		t.Fatal("expected split, got nil")
	}
	if split.Name != "PPL" {
		t.Fatalf("expected 'PPL', got %q", split.Name)
	}
}

func TestCreateUserWithPassword(t *testing.T) {
	cleanDB(t, testDB)

	user, err := CreateUser(testDB, "Test User", "test@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "test@example.com" {
		t.Fatalf("expected test@example.com, got %s", user.Email)
	}
	if user.AuthProvider != "local" {
		t.Fatalf("expected local, got %s", user.AuthProvider)
	}
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	cleanDB(t, testDB)

	CreateUser(testDB, "User 1", "dupe@example.com", "password1")
	_, err := CreateUser(testDB, "User 2", "dupe@example.com", "password2")
	if err == nil {
		t.Fatal("expected error for duplicate email")
	}
}

func TestFindByEmailAndValidatePassword(t *testing.T) {
	cleanDB(t, testDB)

	CreateUser(testDB, "Test", "login@example.com", "correct-password")

	user, err := FindByEmail(testDB, "login@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if user == nil {
		t.Fatal("expected user")
	}

	if !user.ValidatePassword("correct-password") {
		t.Fatal("expected valid password")
	}
	if user.ValidatePassword("wrong-password") {
		t.Fatal("expected invalid password")
	}
}

func TestFindByEmailNotFound(t *testing.T) {
	cleanDB(t, testDB)

	user, err := FindByEmail(testDB, "nobody@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if user != nil {
		t.Fatal("expected nil for missing user")
	}
}

func TestFindOrCreateGoogleUser(t *testing.T) {
	cleanDB(t, testDB)

	user1, err := FindOrCreateGoogleUser(testDB, "google-123", "guser@gmail.com", "Google User")
	if err != nil {
		t.Fatal(err)
	}
	if user1.AuthProvider != "google" {
		t.Fatalf("expected google, got %s", user1.AuthProvider)
	}

	// Calling again returns same user
	user2, _ := FindOrCreateGoogleUser(testDB, "google-123", "guser@gmail.com", "Google User")
	if user1.ID != user2.ID {
		t.Fatal("expected same user ID on second call")
	}
}
