# BarterSwap - API d'échange de compétences

BarterSwap is a Go REST API where time credits are exchanged for services. This
repository currently contains the common application infrastructure and the
Person 1 scope: users, skills, middleware, PostgreSQL setup, and welcome credits.

## Requirements

- Go 1.22 or later
- PostgreSQL
- Docker Compose is optional but provides the quickest local database setup

The PostgreSQL driver is the only external Go dependency. The API uses
`net/http`, `encoding/json`, `context`, and `database/sql` from the standard
library; it does not use an ORM or HTTP framework.

## Installation

Start PostgreSQL:

```bash
docker compose up -d postgres
```

Then start the API:

```bash
go mod tidy
go run .
```

The default server address is `http://localhost:8080`. The application applies
`schema.sql` automatically at startup.

Optional environment variables:

| Variable | Default |
| --- | --- |
| `PORT` | `8080` |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/barterswap?sslmode=disable` |

## Authentication

Protected endpoints use a simple header containing the authenticated user's ID:

```text
X-User-ID: 1
```

Profiles and skills are publicly readable. Updating a profile or its skills
requires the header, and a user may update only their own data.

## Current endpoints

| Method | Path | Authentication | Description |
| --- | --- | --- | --- |
| `GET` | `/healthz` | No | Health check |
| `POST` | `/api/users` | No | Create a user with 10 welcome credits |
| `GET` | `/api/users/{id}` | No | Get a public profile, skills, and balance |
| `PUT` | `/api/users/{id}` | Owner | Replace profile fields |
| `GET` | `/api/users/{id}/skills` | No | Get the user's skills |
| `PUT` | `/api/users/{id}/skills` | Owner | Replace all the user's skills |

The allowed skill levels are `débutant`, `intermédiaire`, and `expert`.

### Create a user

```bash
curl -i -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{"pseudo":"Alice","bio":"Développeuse Go","ville":"Paris"}'
```

### Replace skills

`PUT` replaces the complete skill list; it does not append individual skills.

```bash
curl -i -X PUT http://localhost:8080/api/users/1/skills \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 1" \
  -d '[
    {"nom":"Informatique","niveau":"expert"},
    {"nom":"Cuisine","niveau":"débutant"}
  ]'
```

### Update a profile

```bash
curl -i -X PUT http://localhost:8080/api/users/1 \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 1" \
  -d '{"pseudo":"Alice","bio":"Nouvelle bio","ville":"Lyon"}'
```

### Read a profile

```bash
curl -i http://localhost:8080/api/users/1
```

## Error format

Errors use a consistent JSON structure:

```json
{
  "error": {
    "code": "validation_error",
    "message": "validation error: pseudo is required"
  }
}
```

The shared sentinel errors map to `400`, `401`, `403`, `404`, `409`, or `500`
as appropriate.

## Tests

Run the normal unit and HTTP tests:

```bash
go test -v -cover ./...
go vet ./...
```

An optional PostgreSQL integration test verifies the real `database/sql` store.
It is disabled by default so normal tests do not require a running database:

```bash
RUN_POSTGRES_INTEGRATION=1 \
TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/barterswap?sslmode=disable' \
go test -run TestPostgresUserStoreIntegration -v
```

The test creates one temporary user and deletes that user before finishing. Use
a development database, never a production database.

## Architecture and integration

All `.go` files remain in the single `main` package required by the subject, but
responsibilities are separated:

```text
HTTP request
    -> middleware
    -> user_handler.go
    -> user_service.go
    -> user_store_postgres.go
    -> PostgreSQL
```

Useful integration contracts for the other team members:

- Implement `RouteRegistrar` and pass the handler to `NewApplicationHandler` to
  add service, exchange, review, and statistics routes.
- Call `AuthenticatedUserID(ctx)` to read the user established by the auth
  middleware.
- Call `UserService.UserExists` and `UserService.UserHasSkill` for cross-feature
  validation.
- Reuse `writeJSON`, `decodeJSON`, `writeAPIError`, and the sentinel errors for
  consistent HTTP responses.
- The welcome-credit entry is stored in `credit_transactions` with type `earn`
  and a `NULL` exchange ID. Person 3 can extend this journal for exchange debits,
  earnings, and refunds.

Detailed assignments for the remaining features are in
`WORK_PERSON_2_AND_3.md`.
