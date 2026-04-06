package models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID
	Email     string
	Name      string
	CreatedAt time.Time
}

func EnsureDefaultUser(db *sql.DB) (*User, error) {
	const email = "default@astrid.fit"
	const name = "Astrid User"

	var u User
	err := db.QueryRow(
		`SELECT id, email, name, created_at FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)

	if err == sql.ErrNoRows {
		err = db.QueryRow(
			`INSERT INTO users (email, name) VALUES ($1, $2) RETURNING id, email, name, created_at`,
			email, name,
		).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("insert default user: %w", err)
		}
		return &u, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query default user: %w", err)
	}
	return &u, nil
}
