# Melur API

Go REST API foundation for the Melur local app. It currently provides a health
endpoint and the initial PostgreSQL schema for the MVP.

## Requirements

- Go 1.24 or newer
- Docker Engine with the Compose plugin

## Run locally

```sh
cp .env.example .env
docker compose up -d
set -a
. ./.env
set +a
make migrate-up
make run
```

The API listens on `http://localhost:8080`. Check it with:

```sh
curl http://localhost:8080/health
```

Expected response: `{"status":"ok"}`.

Use `make migrate-down` to roll back the latest migration, `make test` to run
the test suite, and `make docker-down` to stop PostgreSQL. The `.env` file is
local-only; keep secrets out of version control.
