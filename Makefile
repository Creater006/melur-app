.PHONY: run build test fmt docker-up docker-down migrate-up migrate-down seed-dev

run:
	go run ./cmd/server

build:
	go build -o bin/melur-api ./cmd/server

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal

docker-up:
	docker compose up -d

docker-down:
	docker compose down

migrate-up:
	go run ./cmd/migrate -direction=up

migrate-down:
	go run ./cmd/migrate -direction=down

seed-dev:
	go run ./cmd/seed