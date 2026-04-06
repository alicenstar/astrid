package models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type CaloriePlan struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	IsActive  bool
	CreatedAt time.Time
	Days      []CaloriePlanDay
}

type CaloriePlanDay struct {
	ID            uuid.UUID
	PlanID        uuid.UUID
	DayOfWeek     int
	CalorieTarget int
}

var DayNames = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

func ListCaloriePlans(db *sql.DB, userID uuid.UUID) ([]CaloriePlan, error) {
	rows, err := db.Query(
		`SELECT id, user_id, name, is_active, created_at FROM calorie_plans WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list calorie plans: %w", err)
	}
	defer rows.Close()

	var plans []CaloriePlan
	for rows.Next() {
		var p CaloriePlan
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.IsActive, &p.CreatedAt); err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}

	for i := range plans {
		plans[i].Days, err = listPlanDays(db, plans[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return plans, nil
}

func listPlanDays(db *sql.DB, planID uuid.UUID) ([]CaloriePlanDay, error) {
	rows, err := db.Query(
		`SELECT id, plan_id, day_of_week, calorie_target FROM calorie_plan_days WHERE plan_id = $1 ORDER BY day_of_week`,
		planID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var days []CaloriePlanDay
	for rows.Next() {
		var d CaloriePlanDay
		if err := rows.Scan(&d.ID, &d.PlanID, &d.DayOfWeek, &d.CalorieTarget); err != nil {
			return nil, err
		}
		days = append(days, d)
	}
	return days, nil
}

func GetActiveCaloriePlan(db *sql.DB, userID uuid.UUID) (*CaloriePlan, error) {
	var p CaloriePlan
	err := db.QueryRow(
		`SELECT id, user_id, name, is_active, created_at FROM calorie_plans WHERE user_id = $1 AND is_active = true LIMIT 1`,
		userID,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.IsActive, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Days, err = listPlanDays(db, p.ID)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func GetTodayCalorieTarget(db *sql.DB, userID uuid.UUID, dayOfWeek int) (int, error) {
	plan, err := GetActiveCaloriePlan(db, userID)
	if err != nil {
		return 0, err
	}
	if plan == nil {
		return 0, nil
	}
	for _, d := range plan.Days {
		if d.DayOfWeek == dayOfWeek {
			return d.CalorieTarget, nil
		}
	}
	return 0, nil
}

func CreateCaloriePlan(db *sql.DB, userID uuid.UUID, name string, targets map[int]int) (*CaloriePlan, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Auto-activate if no active plan exists
	var hasActive bool
	tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM calorie_plans WHERE user_id = $1 AND is_active = true)`, userID).Scan(&hasActive)

	var p CaloriePlan
	err = tx.QueryRow(
		`INSERT INTO calorie_plans (user_id, name, is_active) VALUES ($1, $2, $3) RETURNING id, user_id, name, is_active, created_at`,
		userID, name, !hasActive,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.IsActive, &p.CreatedAt)
	if err != nil {
		return nil, err
	}

	for day, target := range targets {
		if target <= 0 {
			continue
		}
		var d CaloriePlanDay
		err = tx.QueryRow(
			`INSERT INTO calorie_plan_days (plan_id, day_of_week, calorie_target) VALUES ($1, $2, $3) RETURNING id, plan_id, day_of_week, calorie_target`,
			p.ID, day, target,
		).Scan(&d.ID, &d.PlanID, &d.DayOfWeek, &d.CalorieTarget)
		if err != nil {
			return nil, err
		}
		p.Days = append(p.Days, d)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &p, nil
}

func SetActivePlan(db *sql.DB, userID, planID uuid.UUID) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE calorie_plans SET is_active = false WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE calorie_plans SET is_active = true WHERE id = $1 AND user_id = $2`, planID, userID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func DeleteCaloriePlan(db *sql.DB, planID, userID uuid.UUID) error {
	_, err := db.Exec(`DELETE FROM calorie_plans WHERE id = $1 AND user_id = $2`, planID, userID)
	return err
}
