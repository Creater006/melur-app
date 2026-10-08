package config

import (
	"fmt"
	"os"
)

type Config struct {
	AppEnv      string
	AppPort     string
	DatabaseURL string
	JWTSecret   string
}

func Load() Config {
	appEnv := getEnv("APP_ENV", "development")
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" && appEnv == "development" {
		jwtSecret = "local-development-only-change-me-32-byte-secret"
	}

	return Config{
		AppEnv:      appEnv,
		AppPort:     getEnv("APP_PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://devuser:MelurApp$2026@localhost:5432/nammamelur?sslmode=disable"),
		JWTSecret:   jwtSecret,
	}
}

func (c Config) Address() string {
	return fmt.Sprintf(":%s", c.AppPort)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
