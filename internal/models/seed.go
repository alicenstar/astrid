package models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SeedDemoData populates the demo user's account with sample data.
// Idempotent: clears existing demo data and re-seeds fresh.
func SeedDemoData(db *sql.DB, userID uuid.UUID) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear existing data (cascades handle child records)
	for _, table := range []string{"daily_logs", "workout_logs", "calorie_plans", "workout_splits"} {
		if _, err := tx.Exec(fmt.Sprintf(`DELETE FROM %s WHERE user_id = $1`, table), userID); err != nil {
			return fmt.Errorf("seed cleanup %s: %w", table, err)
		}
	}

	// --- Calorie Plan: "Cut" with training/rest day split ---
	var planID uuid.UUID
	err = tx.QueryRow(
		`INSERT INTO calorie_plans (user_id, name, is_active) VALUES ($1, 'Cut', true) RETURNING id`,
		userID,
	).Scan(&planID)
	if err != nil {
		return fmt.Errorf("seed calorie plan: %w", err)
	}

	// Mon-Fri: 2200 (training), Sat-Sun: 1800 (rest)
	dayTargets := map[int]int{
		0: 1800, // Sun
		1: 2200, // Mon
		2: 2200, // Tue
		3: 2200, // Wed
		4: 2200, // Thu
		5: 2200, // Fri
		6: 1800, // Sat
	}
	for dow, target := range dayTargets {
		_, err = tx.Exec(
			`INSERT INTO calorie_plan_days (plan_id, day_of_week, calorie_target) VALUES ($1, $2, $3)`,
			planID, dow, target,
		)
		if err != nil {
			return fmt.Errorf("seed plan day %d: %w", dow, err)
		}
	}

	// --- Workout Split: Push/Pull/Legs ---
	var splitID uuid.UUID
	err = tx.QueryRow(
		`INSERT INTO workout_splits (user_id, name, is_active) VALUES ($1, 'Push / Pull / Legs', true) RETURNING id`,
		userID,
	).Scan(&splitID)
	if err != nil {
		return fmt.Errorf("seed workout split: %w", err)
	}

	type dayDef struct {
		dow       int
		label     string
		exercises []struct {
			name string
			sets int
			reps int
		}
	}
	days := []dayDef{
		{1, "Push", []struct {
			name string
			sets int
			reps int
		}{
			{"Bench Press", 4, 8},
			{"Overhead Press", 3, 10},
			{"Incline Dumbbell Press", 3, 10},
			{"Lateral Raises", 3, 15},
			{"Tricep Pushdowns", 3, 12},
		}},
		{2, "Pull", []struct {
			name string
			sets int
			reps int
		}{
			{"Barbell Rows", 4, 8},
			{"Pull-ups", 3, 8},
			{"Face Pulls", 3, 15},
			{"Dumbbell Curls", 3, 12},
			{"Hammer Curls", 3, 12},
		}},
		{3, "Legs", []struct {
			name string
			sets int
			reps int
		}{
			{"Squats", 4, 8},
			{"Romanian Deadlifts", 3, 10},
			{"Leg Press", 3, 12},
			{"Leg Curls", 3, 12},
			{"Calf Raises", 4, 15},
		}},
		{4, "Push", []struct {
			name string
			sets int
			reps int
		}{
			{"Overhead Press", 4, 8},
			{"Dumbbell Bench Press", 3, 10},
			{"Cable Flyes", 3, 12},
			{"Lateral Raises", 4, 15},
			{"Overhead Tricep Extension", 3, 12},
		}},
		{5, "Pull", []struct {
			name string
			sets int
			reps int
		}{
			{"Deadlifts", 4, 5},
			{"Lat Pulldowns", 3, 10},
			{"Cable Rows", 3, 12},
			{"Face Pulls", 3, 15},
			{"Barbell Curls", 3, 10},
		}},
	}

	for _, d := range days {
		var dayID uuid.UUID
		err = tx.QueryRow(
			`INSERT INTO split_days (workout_split_id, day_of_week, label, sort_order) VALUES ($1, $2, $3, $4) RETURNING id`,
			splitID, d.dow, d.label, d.dow,
		).Scan(&dayID)
		if err != nil {
			return fmt.Errorf("seed split day %d: %w", d.dow, err)
		}

		for i, ex := range d.exercises {
			_, err = tx.Exec(
				`INSERT INTO planned_exercises (split_day_id, name, sets, reps, sort_order) VALUES ($1, $2, $3, $4, $5)`,
				dayID, ex.name, ex.sets, ex.reps, i,
			)
			if err != nil {
				return fmt.Errorf("seed exercise %s: %w", ex.name, err)
			}
		}
	}

	// --- Meal logs for the past 5 days ---
	today := time.Now().Truncate(24 * time.Hour)

	type mealDef struct {
		name        string
		calories    int
		protein     float64
		fiber       float64
		cholesterol float64
	}

	mealsByDay := [][]mealDef{
		// Today
		{
			{"Oatmeal with banana", 380, 12.0, 6.5, 0},
			{"Grilled chicken salad", 520, 42.0, 5.0, 85},
		},
		// Yesterday
		{
			{"Greek yogurt and granola", 340, 18.0, 3.0, 10},
			{"Turkey wrap", 480, 35.0, 4.0, 65},
			{"Salmon with rice and broccoli", 650, 45.0, 6.0, 70},
			{"Protein shake", 220, 30.0, 2.0, 15},
		},
		// 2 days ago
		{
			{"Scrambled eggs on toast", 420, 24.0, 2.0, 370},
			{"Chicken stir-fry", 580, 40.0, 5.0, 75},
			{"Steak with sweet potato", 700, 50.0, 6.0, 90},
			{"Apple with peanut butter", 280, 8.0, 5.0, 0},
		},
		// 3 days ago
		{
			{"Smoothie bowl", 360, 15.0, 7.0, 5},
			{"Tuna sandwich", 450, 35.0, 3.0, 45},
			{"Pasta with meat sauce", 620, 32.0, 4.0, 60},
			{"Mixed nuts", 200, 6.0, 2.5, 0},
		},
		// 4 days ago
		{
			{"Avocado toast with egg", 400, 16.0, 8.0, 185},
			{"Burrito bowl", 560, 38.0, 9.0, 55},
			{"Grilled chicken with veggies", 480, 44.0, 6.0, 80},
			{"Protein bar", 210, 20.0, 3.0, 5},
		},
	}

	for daysAgo, meals := range mealsByDay {
		date := today.AddDate(0, 0, -daysAgo)
		dateStr := date.Format("2006-01-02")

		var logID uuid.UUID
		err = tx.QueryRow(
			`INSERT INTO daily_logs (user_id, date) VALUES ($1, $2) RETURNING id`,
			userID, dateStr,
		).Scan(&logID)
		if err != nil {
			return fmt.Errorf("seed daily log day-%d: %w", daysAgo, err)
		}

		for _, m := range meals {
			_, err = tx.Exec(
				`INSERT INTO meals (daily_log_id, name, calories, protein_g, fiber_g, cholesterol_mg) VALUES ($1, $2, $3, $4, $5, $6)`,
				logID, m.name, m.calories, m.protein, m.fiber, m.cholesterol,
			)
			if err != nil {
				return fmt.Errorf("seed meal %s: %w", m.name, err)
			}
		}
	}

	// --- Workout logs: completed for today and the past 2 days (3-day streak) ---
	for daysAgo := 0; daysAgo <= 2; daysAgo++ {
		date := today.AddDate(0, 0, -daysAgo)
		dateStr := date.Format("2006-01-02")
		_, err = tx.Exec(
			`INSERT INTO workout_logs (user_id, date, completed) VALUES ($1, $2, true)`,
			userID, dateStr,
		)
		if err != nil {
			return fmt.Errorf("seed workout log day-%d: %w", daysAgo, err)
		}
	}

	return tx.Commit()
}
