package models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type UserProfile struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	HeightCm      *float64
	BirthDate     *time.Time
	Sex           *string
	ActivityLevel *string
	WeightUnit    string
	CreatedAt     time.Time
}

func GetOrCreateProfile(db *sql.DB, userID uuid.UUID) (*UserProfile, error) {
	var p UserProfile
	err := db.QueryRow(
		`SELECT id, user_id, height_cm, birth_date, sex, activity_level, weight_unit, created_at
		 FROM user_profiles WHERE user_id = $1`,
		userID,
	).Scan(&p.ID, &p.UserID, &p.HeightCm, &p.BirthDate, &p.Sex, &p.ActivityLevel, &p.WeightUnit, &p.CreatedAt)
	if err == sql.ErrNoRows {
		err = db.QueryRow(
			`INSERT INTO user_profiles (user_id) VALUES ($1)
			 RETURNING id, user_id, height_cm, birth_date, sex, activity_level, weight_unit, created_at`,
			userID,
		).Scan(&p.ID, &p.UserID, &p.HeightCm, &p.BirthDate, &p.Sex, &p.ActivityLevel, &p.WeightUnit, &p.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("create profile: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}
	return &p, nil
}

func UpdateProfile(db *sql.DB, userID uuid.UUID, heightCm float64, birthDate string, sex string, activityLevel string, weightUnit string) error {
	_, err := db.Exec(
		`UPDATE user_profiles SET height_cm = $1, birth_date = $2, sex = $3, activity_level = $4, weight_unit = $5
		 WHERE user_id = $6`,
		heightCm, birthDate, sex, activityLevel, weightUnit, userID,
	)
	return err
}

// CalculateBMR computes BMR using the Mifflin-St Jeor equation.
// Returns 0 if required fields are missing.
func (p *UserProfile) CalculateBMR(weightKg float64) float64 {
	if p.HeightCm == nil || p.BirthDate == nil || p.Sex == nil {
		return 0
	}
	age := float64(time.Now().Year() - p.BirthDate.Year())
	height := *p.HeightCm

	if *p.Sex == "male" {
		return (10 * weightKg) + (6.25 * height) - (5 * age) + 5
	}
	return (10 * weightKg) + (6.25 * height) - (5 * age) - 161
}

// CalculateTDEE returns BMR * activity multiplier.
func (p *UserProfile) CalculateTDEE(weightKg float64) float64 {
	bmr := p.CalculateBMR(weightKg)
	if bmr == 0 {
		return 0
	}
	multipliers := map[string]float64{
		"sedentary":   1.2,
		"light":       1.375,
		"moderate":    1.55,
		"active":      1.725,
		"very_active": 1.9,
	}
	if p.ActivityLevel == nil {
		return bmr * 1.2
	}
	m, ok := multipliers[*p.ActivityLevel]
	if !ok {
		return bmr * 1.2
	}
	return bmr * m
}
