package models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type BodyMetric struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Date       time.Time
	WeightKg   float64
	BodyFatPct *float64
	MusclePct  *float64
	Notes      string
	CreatedAt  time.Time
}

func CreateBodyMetric(db *sql.DB, userID uuid.UUID, date time.Time, weightKg float64, bodyFatPct *float64, musclePct *float64, notes string) (*BodyMetric, error) {
	dateStr := date.Format("2006-01-02")
	var m BodyMetric
	err := db.QueryRow(
		`INSERT INTO body_metrics (user_id, date, weight_kg, body_fat_pct, muscle_pct, notes)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (user_id, date) DO UPDATE SET weight_kg = $3, body_fat_pct = $4, muscle_pct = $5, notes = $6
		 RETURNING id, user_id, date, weight_kg, body_fat_pct, muscle_pct, notes, created_at`,
		userID, dateStr, weightKg, bodyFatPct, musclePct, notes,
	).Scan(&m.ID, &m.UserID, &m.Date, &m.WeightKg, &m.BodyFatPct, &m.MusclePct, &m.Notes, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create body metric: %w", err)
	}
	return &m, nil
}

func ListBodyMetrics(db *sql.DB, userID uuid.UUID, limit int) ([]BodyMetric, error) {
	rows, err := db.Query(
		`SELECT id, user_id, date, weight_kg, body_fat_pct, muscle_pct, notes, created_at
		 FROM body_metrics WHERE user_id = $1 ORDER BY date DESC LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list body metrics: %w", err)
	}
	defer rows.Close()

	var metrics []BodyMetric
	for rows.Next() {
		var m BodyMetric
		if err := rows.Scan(&m.ID, &m.UserID, &m.Date, &m.WeightKg, &m.BodyFatPct, &m.MusclePct, &m.Notes, &m.CreatedAt); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

func GetLatestBodyMetric(db *sql.DB, userID uuid.UUID) (*BodyMetric, error) {
	var m BodyMetric
	err := db.QueryRow(
		`SELECT id, user_id, date, weight_kg, body_fat_pct, muscle_pct, notes, created_at
		 FROM body_metrics WHERE user_id = $1 ORDER BY date DESC LIMIT 1`,
		userID,
	).Scan(&m.ID, &m.UserID, &m.Date, &m.WeightKg, &m.BodyFatPct, &m.MusclePct, &m.Notes, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest body metric: %w", err)
	}
	return &m, nil
}

func GetWeightTrend(db *sql.DB, userID uuid.UUID, daysAgo int) (*float64, error) {
	var weight float64
	dateStr := time.Now().AddDate(0, 0, -daysAgo).Format("2006-01-02")
	err := db.QueryRow(
		`SELECT weight_kg FROM body_metrics WHERE user_id = $1 AND date <= $2 ORDER BY date DESC LIMIT 1`,
		userID, dateStr,
	).Scan(&weight)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &weight, nil
}

func DeleteBodyMetric(db *sql.DB, metricID, userID uuid.UUID) error {
	_, err := db.Exec(`DELETE FROM body_metrics WHERE id = $1 AND user_id = $2`, metricID, userID)
	return err
}
