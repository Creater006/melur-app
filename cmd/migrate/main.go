package main

import (
	"database/sql"
	"flag"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	direction := flag.String("direction", "up", "migration direction: up or down")
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("failed to set migration dialect: %v", err)
	}

	switch *direction {
	case "up":
		err = goose.Up(db, "migrations")
	case "down":
		err = goose.Down(db, "migrations")
	default:
		log.Fatalf("unknown migration direction: %s", *direction)
	}
	if err != nil {
		log.Fatalf("migration %s failed: %v", *direction, err)
	}
	log.Printf("migration %s completed", *direction)
}
