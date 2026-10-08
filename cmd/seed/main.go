package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nammmelur/melur-app/internal/auth"
	"github.com/nammmelur/melur-app/internal/config"
	"github.com/nammmelur/melur-app/internal/database"
)

func main() {
	if err := seed(); err != nil {
		log.Fatal(err)
	}
}

func seed() error {
	cfg := config.Load()
	if cfg.AppEnv != "development" {
		return fmt.Errorf("sample account seeding is allowed only when APP_ENV=development")
	}

	email := getEnv("SEED_EMAIL", "sample@melur.local")
	phone := getEnv("SEED_PHONE", "9000000000")
	password, hasCustomPassword := os.LookupEnv("SEED_PASSWORD")
	if !hasCustomPassword || password == "" {
		password = "12345"
		hasCustomPassword = false
	}
	if hasCustomPassword && len(password) < 5 {
		return fmt.Errorf("SEED_PASSWORD must be at least 5 characters")
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash sample password: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin sample seed transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO users (name, email, phone, password_hash)
		VALUES ('Melur Sample User', $1, $2, $3)
		ON CONFLICT (phone) DO UPDATE SET
			name = EXCLUDED.name,
			email = EXCLUDED.email,
			password_hash = EXCLUDED.password_hash,
			is_active = TRUE,
			deleted_at = NULL`, email, phone, passwordHash)
	if err != nil {
		return fmt.Errorf("insert sample user: %w", err)
	}

	var userID string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM users WHERE phone = $1 AND deleted_at IS NULL`, phone).Scan(&userID); err != nil {
		return fmt.Errorf("load sample user: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE name = 'user'
		ON CONFLICT DO NOTHING`, userID)
	if err != nil {
		return fmt.Errorf("assign sample user role: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit sample seed transaction: %w", err)
	}

	log.Printf("development sample account ready: userdata=%s (password from SEED_PASSWORD or development default)", phone)
	return nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
