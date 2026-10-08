# Melur API

Go REST API foundation for the Melur local app. It currently provides a health
endpoint, authentication, and the initial PostgreSQL schema for the MVP.

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

## Development login

Create a local sample account after migrations:

```sh
make seed-dev
```

The development-only default credentials are `9000000000` and
`12345`. Override them with `SEED_PHONE`, `SEED_EMAIL`, and `SEED_PASSWORD`.
Custom seed passwords must be at least 5 characters. The seed command refuses
to run unless `APP_ENV=development`; never use this sample password in production.

Login with either phone or email:

```sh
curl -X POST http://localhost:8080/api/v1/auth/login \
	-H 'Content-Type: application/json' \
	-d '{"userdata":"9000000000","password":"12345"}'
```

The response contains a 15-minute HS256 access token and a 30-day opaque refresh
token. Only the SHA-256 hash of the refresh token is stored in PostgreSQL.
Set a private `JWT_SECRET` of at least 32 bytes outside development.

## Logout and password reset

Apply the password-reset token migration before using these endpoints:

```sh
make migrate-up
```

Logout revokes the presented refresh token and returns `204 No Content`:

```sh
curl -X POST http://localhost:8080/api/v1/auth/logout \
	-H 'Content-Type: application/json' \
	-d '{"refresh_token":"<refresh_token>"}'
```

Request a reset with the same phone-or-email `userdata` used for login:

```sh
curl -X POST http://localhost:8080/api/v1/auth/forgot-password \
	-H 'Content-Type: application/json' \
	-d '{"userdata":"9000000000"}'
```

The response is generic to avoid revealing whether the account exists. In
development it also includes a one-hour `reset_token` for local testing. Submit
that token with a new password of at least 5 characters:

```sh
curl -X POST http://localhost:8080/api/v1/auth/reset-password \
	-H 'Content-Type: application/json' \
	-d '{"reset_token":"<reset_token>","new_password":"<new-password>"}'
```

Reset tokens are single-use and stored as hashes. A successful reset revokes
all existing refresh tokens. Production password-reset delivery needs an
SMS/email sender configured; without one the endpoint fails closed with `503`.

Use `make migrate-down` to roll back the latest migration, `make test` to run
the test suite, and `make docker-down` to stop PostgreSQL. The `.env` file is
local-only; keep secrets out of version control.
