package models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           uuid.UUID
	Email        string
	Name         string
	PasswordHash *string
	AuthProvider string
	GoogleID     *string
	CreatedAt    time.Time
}

func (u *User) ValidatePassword(password string) bool {
	if u.PasswordHash == nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(password)) == nil
}

func EnsureDefaultUser(db *sql.DB) (*User, error) {
	const email = "default@astrid.fit"
	const name = "Astrid User"

	var u User
	err := db.QueryRow(
		`INSERT INTO users (email, name) VALUES ($1, $2)
		 ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id, email, name, password_hash, auth_provider, google_id, created_at`,
		email, name,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.AuthProvider, &u.GoogleID, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("ensure default user: %w", err)
	}
	return &u, nil
}

func CreateUser(db *sql.DB, name, email, password string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	hashStr := string(hash)

	var u User
	err = db.QueryRow(
		`INSERT INTO users (email, name, password_hash, auth_provider)
		 VALUES ($1, $2, $3, 'local')
		 RETURNING id, email, name, password_hash, auth_provider, google_id, created_at`,
		email, name, hashStr,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.AuthProvider, &u.GoogleID, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

func FindByEmail(db *sql.DB, email string) (*User, error) {
	var u User
	err := db.QueryRow(
		`SELECT id, email, name, password_hash, auth_provider, google_id, created_at FROM users WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.AuthProvider, &u.GoogleID, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func FindOrCreateGoogleUser(db *sql.DB, googleID, email, name string) (*User, error) {
	var u User
	err := db.QueryRow(
		`SELECT id, email, name, password_hash, auth_provider, google_id, created_at FROM users WHERE google_id = $1`,
		googleID,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.AuthProvider, &u.GoogleID, &u.CreatedAt)
	if err == nil {
		return &u, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	err = db.QueryRow(
		`INSERT INTO users (email, name, auth_provider, google_id)
		 VALUES ($1, $2, 'google', $3)
		 RETURNING id, email, name, password_hash, auth_provider, google_id, created_at`,
		email, name, googleID,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.AuthProvider, &u.GoogleID, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create google user: %w", err)
	}
	return &u, nil
}
