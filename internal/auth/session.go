package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const SessionTTL = 7 * 24 * time.Hour

type SessionData struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func CreateSession(rdb *redis.Client, userID uuid.UUID, email string) (string, error) {
	sessionID := uuid.New().String()
	data := SessionData{
		UserID:    userID,
		Email:     email,
		CreatedAt: time.Now(),
	}
	b, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal session: %w", err)
	}
	key := "session:" + sessionID
	if err := rdb.Set(context.Background(), key, b, SessionTTL).Err(); err != nil {
		return "", fmt.Errorf("store session: %w", err)
	}
	return sessionID, nil
}

func GetSession(rdb *redis.Client, sessionID string) (*SessionData, error) {
	key := "session:" + sessionID
	b, err := rdb.Get(context.Background(), key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var data SessionData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func DeleteSession(rdb *redis.Client, sessionID string) error {
	return rdb.Del(context.Background(), "session:"+sessionID).Err()
}
