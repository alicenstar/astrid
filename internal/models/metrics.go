package models

import "database/sql"

type AppMetrics struct {
	UserCount    int
	PlanCount    int
	MealCount    int
	WorkoutCount int
}

func GetAppMetrics(db *sql.DB) (AppMetrics, error) {
	var m AppMetrics
	err := db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM calorie_plans),
			(SELECT COUNT(*) FROM meals),
			(SELECT COUNT(*) FROM workout_logs)
	`).Scan(&m.UserCount, &m.PlanCount, &m.MealCount, &m.WorkoutCount)
	return m, err
}
