package config

import (
	"fmt"
	"os"
)

type Config struct {
	AppEnv      string
	AppPort     string
	DatabaseURL string
}

func Load() Config {
	return Config{
		AppEnv:      getEnv("APP_ENV", "development"),
		AppPort:     getEnv("APP_PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://devuser:MelurApp@2026@localhost:5432/nammamelur?sslmode=disable"),
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
