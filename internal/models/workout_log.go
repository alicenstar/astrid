package models

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type WorkoutLog struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Date       time.Time
	SplitDayID *uuid.UUID
	Completed  bool
	Notes      string
	CreatedAt  time.Time
}

func GetWorkoutLog(db *sql.DB, userID uuid.UUID, date time.Time) (*WorkoutLog, error) {
	dateStr := date.Format("2006-01-02")
	var wl WorkoutLog
	err := db.QueryRow(
		`SELECT id, user_id, date, split_day_id, completed, notes, created_at
		 FROM workout_logs WHERE user_id = $1 AND date = $2`,
		userID, dateStr,
	).Scan(&wl.ID, &wl.UserID, &wl.Date, &wl.SplitDayID, &wl.Completed, &wl.Notes, &wl.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &wl, nil
}

func CacheStreak(rdb *redis.Client, userID uuid.UUID, streak int) {
	ctx := context.Background()
	key := fmt.Sprintf("streak:%s:workouts", userID)
	rdb.Set(ctx, key, strconv.Itoa(streak), 24*time.Hour)
}

func InvalidateStreakCache(rdb *redis.Client, userID uuid.UUID) {
	ctx := context.Background()
	key := fmt.Sprintf("streak:%s:workouts", userID)
	rdb.Del(ctx, key)
}

func GetCachedStreak(rdb *redis.Client, userID uuid.UUID) (int, bool) {
	ctx := context.Background()
	key := fmt.Sprintf("streak:%s:workouts", userID)
	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		return 0, false
	}
	streak, _ := strconv.Atoi(val)
	return streak, true
}

func ToggleWorkoutComplete(db *sql.DB, rdb *redis.Client, userID uuid.UUID, date time.Time, splitDayID *uuid.UUID) error {
	dateStr := date.Format("2006-01-02")
	existing, err := GetWorkoutLog(db, userID, date)
	if err != nil {
		return err
	}

	if existing != nil {
		_, err = db.Exec(
			`UPDATE workout_logs SET completed = NOT completed WHERE id = $1`,
			existing.ID,
		)
	} else {
		_, err = db.Exec(
			`INSERT INTO workout_logs (user_id, date, split_day_id, completed) VALUES ($1, $2, $3, true)`,
			userID, dateStr, splitDayID,
		)
	}
	if err == nil && rdb != nil {
		InvalidateStreakCache(rdb, userID)
	}
	return err
}

func GetWorkoutStreak(db *sql.DB, rdb *redis.Client, userID uuid.UUID) (int, error) {
	// Check cache first
	if rdb != nil {
		if cached, ok := GetCachedStreak(rdb, userID); ok {
			return cached, nil
		}
	}

	rows, err := db.Query(
		`SELECT date FROM workout_logs WHERE user_id = $1 AND completed = true ORDER BY date DESC`,
		userID,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	streak := 0
	expected := time.Now().Truncate(24 * time.Hour)

	for rows.Next() {
		var date time.Time
		if err := rows.Scan(&date); err != nil {
			return 0, err
		}
		date = date.Truncate(24 * time.Hour)

		if date.Equal(expected) {
			streak++
			expected = expected.AddDate(0, 0, -1)
		} else if date.Equal(expected.AddDate(0, 0, -1)) {
			// Allow checking yesterday if today hasn't been logged yet
			if streak == 0 {
				expected = date
				streak++
				expected = expected.AddDate(0, 0, -1)
			} else {
				break
			}
		} else {
			break
		}
	}

	// Cache the result
	if rdb != nil {
		CacheStreak(rdb, userID, streak)
	}
	return streak, nil
}
