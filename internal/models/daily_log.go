package models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type DailyLog struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Date      time.Time
	Notes     string
	CreatedAt time.Time
	Meals     []Meal
}

type Meal struct {
	ID            uuid.UUID
	DailyLogID    uuid.UUID
	Name          string
	Calories      int
	ProteinG      float64
	FiberG        float64
	CholesterolMg float64
	LoggedAt      time.Time
}

type DailySummary struct {
	TotalCalories    int
	TotalProtein     float64
	TotalFiber       float64
	TotalCholesterol float64
	CalorieTarget    int
}

func GetOrCreateDailyLog(db *sql.DB, userID uuid.UUID, date time.Time) (*DailyLog, error) {
	dateStr := date.Format("2006-01-02")
	var log DailyLog
	err := db.QueryRow(
		`SELECT id, user_id, date, notes, created_at FROM daily_logs WHERE user_id = $1 AND date = $2`,
		userID, dateStr,
	).Scan(&log.ID, &log.UserID, &log.Date, &log.Notes, &log.CreatedAt)

	if err == sql.ErrNoRows {
		err = db.QueryRow(
			`INSERT INTO daily_logs (user_id, date) VALUES ($1, $2) RETURNING id, user_id, date, notes, created_at`,
			userID, dateStr,
		).Scan(&log.ID, &log.UserID, &log.Date, &log.Notes, &log.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("create daily log: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("get daily log: %w", err)
	}

	log.Meals, err = ListMeals(db, log.ID)
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func ListMeals(db *sql.DB, dailyLogID uuid.UUID) ([]Meal, error) {
	rows, err := db.Query(
		`SELECT id, daily_log_id, name, calories, protein_g, fiber_g, cholesterol_mg, logged_at
		 FROM meals WHERE daily_log_id = $1 ORDER BY logged_at`,
		dailyLogID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var meals []Meal
	for rows.Next() {
		var m Meal
		if err := rows.Scan(&m.ID, &m.DailyLogID, &m.Name, &m.Calories, &m.ProteinG, &m.FiberG, &m.CholesterolMg, &m.LoggedAt); err != nil {
			return nil, err
		}
		meals = append(meals, m)
	}
	return meals, nil
}

func CreateMeal(db *sql.DB, dailyLogID uuid.UUID, name string, calories int, protein, fiber, cholesterol float64) (*Meal, error) {
	var m Meal
	err := db.QueryRow(
		`INSERT INTO meals (daily_log_id, name, calories, protein_g, fiber_g, cholesterol_mg)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, daily_log_id, name, calories, protein_g, fiber_g, cholesterol_mg, logged_at`,
		dailyLogID, name, calories, protein, fiber, cholesterol,
	).Scan(&m.ID, &m.DailyLogID, &m.Name, &m.Calories, &m.ProteinG, &m.FiberG, &m.CholesterolMg, &m.LoggedAt)
	if err != nil {
		return nil, fmt.Errorf("create meal: %w", err)
	}
	return &m, nil
}

func DeleteMeal(db *sql.DB, mealID uuid.UUID) error {
	_, err := db.Exec(`DELETE FROM meals WHERE id = $1`, mealID)
	return err
}

func GetDailySummary(db *sql.DB, userID uuid.UUID, date time.Time, dayOfWeek int) (*DailySummary, error) {
	dateStr := date.Format("2006-01-02")
	var s DailySummary
	err := db.QueryRow(
		`SELECT COALESCE(SUM(m.calories), 0), COALESCE(SUM(m.protein_g), 0),
		        COALESCE(SUM(m.fiber_g), 0), COALESCE(SUM(m.cholesterol_mg), 0)
		 FROM meals m
		 JOIN daily_logs dl ON m.daily_log_id = dl.id
		 WHERE dl.user_id = $1 AND dl.date = $2`,
		userID, dateStr,
	).Scan(&s.TotalCalories, &s.TotalProtein, &s.TotalFiber, &s.TotalCholesterol)
	if err != nil {
		return nil, err
	}

	s.CalorieTarget, err = GetTodayCalorieTarget(db, userID, dayOfWeek)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
