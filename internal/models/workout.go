package models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type WorkoutSplit struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	IsActive  bool
	CreatedAt time.Time
	Days      []SplitDay
}

type SplitDay struct {
	ID             uuid.UUID
	WorkoutSplitID uuid.UUID
	DayOfWeek      int
	Label          string
	SortOrder      int
	Exercises      []PlannedExercise
}

type PlannedExercise struct {
	ID         uuid.UUID
	SplitDayID uuid.UUID
	Name       string
	Sets       int
	Reps       int
	Notes      string
	SortOrder  int
}

func ListWorkoutSplits(db *sql.DB, userID uuid.UUID) ([]WorkoutSplit, error) {
	rows, err := db.Query(
		`SELECT id, user_id, name, is_active, created_at FROM workout_splits WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var splits []WorkoutSplit
	for rows.Next() {
		var s WorkoutSplit
		if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &s.IsActive, &s.CreatedAt); err != nil {
			return nil, err
		}
		splits = append(splits, s)
	}

	for i := range splits {
		splits[i].Days, err = listSplitDays(db, splits[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return splits, nil
}

func listSplitDays(db *sql.DB, splitID uuid.UUID) ([]SplitDay, error) {
	rows, err := db.Query(
		`SELECT id, workout_split_id, day_of_week, label, sort_order FROM split_days WHERE workout_split_id = $1 ORDER BY day_of_week`,
		splitID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var days []SplitDay
	for rows.Next() {
		var d SplitDay
		if err := rows.Scan(&d.ID, &d.WorkoutSplitID, &d.DayOfWeek, &d.Label, &d.SortOrder); err != nil {
			return nil, err
		}
		days = append(days, d)
	}

	for i := range days {
		days[i].Exercises, err = listExercises(db, days[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return days, nil
}

func listExercises(db *sql.DB, splitDayID uuid.UUID) ([]PlannedExercise, error) {
	rows, err := db.Query(
		`SELECT id, split_day_id, name, sets, reps, notes, sort_order FROM planned_exercises WHERE split_day_id = $1 ORDER BY sort_order`,
		splitDayID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exercises []PlannedExercise
	for rows.Next() {
		var e PlannedExercise
		if err := rows.Scan(&e.ID, &e.SplitDayID, &e.Name, &e.Sets, &e.Reps, &e.Notes, &e.SortOrder); err != nil {
			return nil, err
		}
		exercises = append(exercises, e)
	}
	return exercises, nil
}

func GetActiveWorkoutSplit(db *sql.DB, userID uuid.UUID) (*WorkoutSplit, error) {
	var s WorkoutSplit
	err := db.QueryRow(
		`SELECT id, user_id, name, is_active, created_at FROM workout_splits WHERE user_id = $1 AND is_active = true LIMIT 1`,
		userID,
	).Scan(&s.ID, &s.UserID, &s.Name, &s.IsActive, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.Days, err = listSplitDays(db, s.ID)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func GetTodaySplitDay(db *sql.DB, userID uuid.UUID, dayOfWeek int) (*SplitDay, error) {
	split, err := GetActiveWorkoutSplit(db, userID)
	if err != nil || split == nil {
		return nil, err
	}
	for _, d := range split.Days {
		if d.DayOfWeek == dayOfWeek {
			return &d, nil
		}
	}
	return nil, nil
}

func CreateWorkoutSplit(db *sql.DB, userID uuid.UUID, name string, days map[int]string) (*WorkoutSplit, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Auto-activate if no active split exists
	var hasActive bool
	tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM workout_splits WHERE user_id = $1 AND is_active = true)`, userID).Scan(&hasActive)

	var s WorkoutSplit
	err = tx.QueryRow(
		`INSERT INTO workout_splits (user_id, name, is_active) VALUES ($1, $2, $3) RETURNING id, user_id, name, is_active, created_at`,
		userID, name, !hasActive,
	).Scan(&s.ID, &s.UserID, &s.Name, &s.IsActive, &s.CreatedAt)
	if err != nil {
		return nil, err
	}

	for day, label := range days {
		if label == "" {
			continue
		}
		var d SplitDay
		err = tx.QueryRow(
			`INSERT INTO split_days (workout_split_id, day_of_week, label, sort_order) VALUES ($1, $2, $3, $4) RETURNING id, workout_split_id, day_of_week, label, sort_order`,
			s.ID, day, label, day,
		).Scan(&d.ID, &d.WorkoutSplitID, &d.DayOfWeek, &d.Label, &d.SortOrder)
		if err != nil {
			return nil, err
		}
		s.Days = append(s.Days, d)
	}

	return &s, tx.Commit()
}

func SetActiveSplit(db *sql.DB, userID, splitID uuid.UUID) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE workout_splits SET is_active = false WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE workout_splits SET is_active = true WHERE id = $1 AND user_id = $2`, splitID, userID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func GetWorkoutSplit(db *sql.DB, splitID, userID uuid.UUID) (*WorkoutSplit, error) {
	var s WorkoutSplit
	err := db.QueryRow(
		`SELECT id, user_id, name, is_active, created_at FROM workout_splits WHERE id = $1 AND user_id = $2`,
		splitID, userID,
	).Scan(&s.ID, &s.UserID, &s.Name, &s.IsActive, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.Days, err = listSplitDays(db, s.ID)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func UpdateWorkoutSplit(db *sql.DB, splitID, userID uuid.UUID, name string, days map[int]string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE workout_splits SET name = $1 WHERE id = $2 AND user_id = $3`, name, splitID, userID)
	if err != nil {
		return err
	}

	// Delete existing days (cascades to exercises) and re-insert
	_, err = tx.Exec(`DELETE FROM split_days WHERE workout_split_id = $1`, splitID)
	if err != nil {
		return err
	}

	for day, label := range days {
		if label == "" {
			continue
		}
		_, err = tx.Exec(
			`INSERT INTO split_days (workout_split_id, day_of_week, label, sort_order) VALUES ($1, $2, $3, $4)`,
			splitID, day, label, day,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func DeleteWorkoutSplit(db *sql.DB, splitID, userID uuid.UUID) error {
	_, err := db.Exec(`DELETE FROM workout_splits WHERE id = $1 AND user_id = $2`, splitID, userID)
	return err
}

func AddExercise(db *sql.DB, splitDayID uuid.UUID, name string, sets, reps int, notes string) (*PlannedExercise, error) {
	var maxOrder int
	db.QueryRow(`SELECT COALESCE(MAX(sort_order), -1) FROM planned_exercises WHERE split_day_id = $1`, splitDayID).Scan(&maxOrder)

	var e PlannedExercise
	err := db.QueryRow(
		`INSERT INTO planned_exercises (split_day_id, name, sets, reps, notes, sort_order)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, split_day_id, name, sets, reps, notes, sort_order`,
		splitDayID, name, sets, reps, notes, maxOrder+1,
	).Scan(&e.ID, &e.SplitDayID, &e.Name, &e.Sets, &e.Reps, &e.Notes, &e.SortOrder)
	if err != nil {
		return nil, fmt.Errorf("add exercise: %w", err)
	}
	return &e, nil
}

func DeleteExercise(db *sql.DB, exerciseID uuid.UUID) error {
	_, err := db.Exec(`DELETE FROM planned_exercises WHERE id = $1`, exerciseID)
	return err
}
