package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port              string
	DatabaseURL       string
	RedisURL          string
	GoogleClientID    string
	GoogleSecret      string
	GoogleRedirectURL string
	ReplicatedSDKURL  string
	AppVersion        string
	StreaksEnabled    bool
}

func Load() *Config {
	return &Config{
		Port:              getEnv("PORT", "8080"),
		AppVersion:        getEnv("APP_VERSION", "dev"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://astrid:astrid@localhost:5432/astrid?sslmode=disable"),
		RedisURL:          getEnv("REDIS_URL", "redis://localhost:6379/0"),
		GoogleClientID:    os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleSecret:      os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL: os.Getenv("GOOGLE_REDIRECT_URL"),
		ReplicatedSDKURL:  getEnv("REPLICATED_SDK_URL", ""),
		// STREAKS_ENABLED is set by Helm from the EC config screen.
		// Anything other than explicit "false" preserves existing SDK-check behavior.
		StreaksEnabled: os.Getenv("STREAKS_ENABLED") != "false",
	}
}

func (c *Config) GoogleOAuthEnabled() bool {
	return c.GoogleClientID != "" && c.GoogleSecret != ""
}

func (c *Config) Addr() string {
	return fmt.Sprintf(":%s", c.Port)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
