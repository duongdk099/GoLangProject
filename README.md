# BarterSwap - API d'échange de compétences

BarterSwap is a Go REST API where time credits are exchanged for services. This
repository contains the common application infrastructure (Person 1: users,
skills, middleware, PostgreSQL setup, welcome credits), the Person 2 scope
(service advertisements, reviews, and user statistics), and the Person 3 scope
(the exchange lifecycle and the credit ledger). See
[Exchanges and the credit ledger](#exchanges-and-the-credit-ledger-person-3)
for the transactional design.

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
go run ./cmd/server
```

The default server address is `http://localhost:8080`. The application applies
the embedded schema (`internal/database/schema.sql`) automatically at startup.

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
| `GET` | `/api/services` | No | List services, filterable by `categorie`, `ville`, `search` |
| `POST` | `/api/services` | Yes | Publish a service for one of your skills |
| `GET` | `/api/services/{id}` | No | Get one service |
| `PUT` | `/api/services/{id}` | Owner | Replace a service's fields |
| `DELETE` | `/api/services/{id}` | Owner | Delete a service |
| `POST` | `/api/exchanges` | Yes | Request an exchange for a service |
| `GET` | `/api/exchanges` | Yes | List your exchanges, filterable by `status` |
| `GET` | `/api/exchanges/{id}` | Participant | Get one exchange |
| `PUT` | `/api/exchanges/{id}/accept` | Owner | Accept a pending request (blocks the requester's credits) |
| `PUT` | `/api/exchanges/{id}/reject` | Owner | Reject a pending request |
| `PUT` | `/api/exchanges/{id}/complete` | Requester | Confirm completion (releases credits to the owner) |
| `PUT` | `/api/exchanges/{id}/cancel` | Participant | Cancel; refunds the requester if already accepted |
| `POST` | `/api/exchanges/{id}/review` | Yes | Review a completed exchange (see note below) |
| `GET` | `/api/users/{id}/reviews` | No | Reviews received by a user |
| `GET` | `/api/services/{id}/reviews` | No | Reviews left on a service |
| `GET` | `/api/users/{id}/stats` | No | Aggregated statistics for a user |

The allowed skill levels are `débutant`, `intermédiaire`, and `expert`. The
allowed service categories are `Informatique`, `Jardinage`, `Bricolage`,
`Cuisine`, `Musique`, `Langues`, `Sport`, `Tutorat`, `Demenagement`,
`Photographie`, `Animalier`, `Couture`, `Autre`.

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

### Publish a service

The category must match one of the authenticated user's skills.

```bash
curl -i -X POST http://localhost:8080/api/services \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 1" \
  -d '{
    "titre":"Cours de Go pour débutants",
    "description":"Deux heures de programmation en binôme",
    "categorie":"Informatique",
    "duree_minutes":120,
    "credits":2,
    "ville":"Paris"
  }'
```

### List and filter services

```bash
curl -i "http://localhost:8080/api/services?categorie=Informatique&ville=Paris"
curl -i "http://localhost:8080/api/services?search=Go"
```

### Update or delete a service

Only the provider who published the service may modify or delete it.

```bash
curl -i -X PUT http://localhost:8080/api/services/1 \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 1" \
  -d '{
    "titre":"Cours de Go avancé",
    "categorie":"Informatique",
    "duree_minutes":90,
    "credits":3,
    "actif":true
  }'

curl -i -X DELETE http://localhost:8080/api/services/1 -H "X-User-ID: 1"
```

### Request and drive an exchange

The requester comes from authentication; the owner and price are resolved from
the service, never trusted from the client. A user cannot request their own
service, must have enough credits, and a service can hold only one `pending` or
`accepted` exchange at a time (a second request returns `409`).

```bash
# User 2 requests service 1 (owned by user 1)
curl -i -X POST http://localhost:8080/api/exchanges \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 2" \
  -d '{"service_id":1}'

# List your exchanges, optionally filtered by status
curl -i "http://localhost:8080/api/exchanges" -H "X-User-ID: 2"
curl -i "http://localhost:8080/api/exchanges?status=accepted" -H "X-User-ID: 2"

# Get one exchange (participants only)
curl -i http://localhost:8080/api/exchanges/1 -H "X-User-ID: 2"

# The owner accepts (blocks the requester's credits) or rejects
curl -i -X PUT http://localhost:8080/api/exchanges/1/accept -H "X-User-ID: 1"
curl -i -X PUT http://localhost:8080/api/exchanges/1/reject -H "X-User-ID: 1"

# The requester confirms completion (releases the credits to the owner)
curl -i -X PUT http://localhost:8080/api/exchanges/1/complete -H "X-User-ID: 2"

# Either participant can cancel; an accepted exchange refunds the requester
curl -i -X PUT http://localhost:8080/api/exchanges/1/cancel -H "X-User-ID: 2"
```

### Review a completed exchange

Only the requester or the owner of a `completed` exchange may leave one
review each; the target is resolved automatically as the other participant.

```bash
curl -i -X POST http://localhost:8080/api/exchanges/1/review \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 1" \
  -d '{"note":5,"commentaire":"Ponctuel et de bon conseil"}'
```

### Read reviews and statistics

```bash
curl -i http://localhost:8080/api/users/1/reviews
curl -i http://localhost:8080/api/services/1/reviews
curl -i http://localhost:8080/api/users/1/stats
```

## Exchanges and the credit ledger (Person 3)

The exchange lifecycle and the credit journal are implemented in
`internal/exchanges` and `internal/credits`. `*exchanges.UseCases` satisfies the
two consumer-side interfaces the reviews and statistics features declared —
`reviews.ExchangeLookup` (`GetExchange`) and `stats.ExchangeStatsProvider`
(`CountCompletedExchanges`) — so `cmd/server/main.go` wires the real store into
both consumers with no change to their code, exactly as the interface contracts
intended.

### Lifecycle and who does what

```text
pending -> accepted -> completed
   |          |
   v          v
rejected   cancelled   (a pending exchange can also be cancelled)
```

- **Create** (`POST /api/exchanges`): the requester is the authenticated user;
  the owner and price come from the service, never the client. A user cannot
  request their own service, the service must be active, the requester must
  have enough credits, and a service can hold at most one `pending` or
  `accepted` exchange — a second request returns `409`.
- **Accept** (owner only): moves `pending -> accepted` and **blocks** the
  service price from the requester as a `spend` entry, inside one transaction.
- **Reject** (owner only): moves `pending -> rejected`. No credit was blocked,
  so nothing is refunded.
- **Complete** (**requester** only): moves `accepted -> completed` and
  **releases** the blocked credits to the owner as an `earn` entry. The team's
  decision is that the requester — the party who received the service —
  confirms completion, since that is what releases their own blocked credits
  and prevents an owner from crediting themselves prematurely.
- **Cancel** (either participant): a `pending` exchange is simply cancelled; an
  `accepted` exchange is cancelled **and refunds** the requester as a `refund`
  entry.

### Transactional guarantees

- Every status change that moves credits (`accept`, `complete`, cancellation of
  an accepted exchange) runs in a single `database/sql` transaction that locks
  the exchange row `FOR UPDATE`, re-checks its state (and the requester's
  balance on `accept`), records the credit movement, and updates the status.
  The status change and its credit movement commit or roll back together.
- No Go mutex is used. Concurrency is handled entirely in the database:
  - A partial unique index (`idx_exchanges_active_service`) makes two
    simultaneous requests for one service race at the database; the losing
    `INSERT` fails and is surfaced as `409 Conflict`.
  - A partial unique index on `credit_transactions (exchange_id, type)`
    guarantees a debit, earning, or refund can never be recorded twice for the
    same exchange and operation.
  - Status guards (`WHERE ... AND status = <expected>`) plus the row lock mean a
    repeated transition is rejected and never transfers credits a second time.

### Credit journal

`internal/credits` is a small library (not an HTTP feature) that owns the
append-only journal. A balance is the sum of a user's entries, never a mutable
column. Its `Record`/`Balance` helpers accept an `Execer` satisfied by both
`*sql.DB` and `*sql.Tx`, so the exchange store records movements **inside** the
transaction that drives the status change. The welcome credit is the same
journal contract: an `earn` entry with a `NULL` exchange ID (Person 1 records it
when a user is created). The internal `credits_cost` column on `exchanges`
snapshots the service price at request time, so the amount blocked on
acceptance is exactly the amount released on completion or refunded on
cancellation even if the provider later edits the price.

The `reviews` table stores `exchange_id` without a foreign key (same pattern as
`credit_transactions`), plus a denormalized `service_id` snapshot captured from
the exchange at review creation time, so `GET /api/services/{id}/reviews` does
not need a live join against `exchanges`.

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

The shared sentinel errors (`httpapi.ErrValidation`, `ErrUnauthorized`,
`ErrForbidden`, `ErrNotFound`, `ErrConflict`) map to `400`, `401`, `403`,
`404`, `409`, or `500` as appropriate.

## Tests

Run the normal unit and HTTP tests:

```bash
go test -v -cover ./...
go vet ./...
```

Optional PostgreSQL integration tests verify the real `database/sql` stores
for users, services, reviews, and statistics. They are disabled by default so
normal tests do not require a running database:

```bash
RUN_POSTGRES_INTEGRATION=1 \
TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/barterswap?sslmode=disable' \
go test ./... -run Integration -p 1 -v
```

Run the integration packages serially with `-p 1`: every package's integration
test applies the schema, and PostgreSQL `CREATE TABLE IF NOT EXISTS` is not safe
under concurrent execution (parallel migrations collide on `pg_type`). Each test
creates a small number of temporary rows and deletes them before finishing. Use
a development database, never a production database.

See [`COVERAGE.md`](COVERAGE.md) for a per-package coverage report and an
explanation of exactly which statements are not covered and why.

## Architecture and integration

The repository is organized like the Go project shown in the course
(Module 8): a thin `cmd/` entry point, one package per feature under
`internal/` (not importable outside this module), and a project-agnostic,
reusable package under `pkg/`. This keeps the codebase clean as the team
grows the project — no giant flat directory of files — while every
responsibility (HTTP exposure, business logic, storage) stays clearly
separated inside each feature package, exactly as required by the subject.

```text
barterswap/
├── cmd/
│   └── server/
│       └── main.go            # wires config, database, and every feature
├── internal/
│   ├── config/       # environment configuration
│   ├── database/     # PostgreSQL connection + embedded schema.sql
│   ├── testutil/     # shared HTTP test helper (PerformRequest)
│   ├── users/        # Person 1: accounts, skills, welcome credits
│   ├── services/     # Person 2: service advertisements
│   ├── reviews/      # Person 2: reviews on completed exchanges
│   ├── stats/        # Person 2: aggregated user dashboard
│   ├── exchanges/    # Person 3: exchange lifecycle (transactional)
│   └── credits/      # Person 3: append-only credit journal (library)
├── pkg/
│   └── httpapi/      # reusable HTTP plumbing (see below)
├── go.mod / go.sum
├── compose.yaml
└── README.md
```

`pkg/httpapi` is deliberately dependency-free with respect to BarterSwap
domain types — it only knows about `net/http` — which is what makes it
reusable across every feature package without import cycles: sentinel
errors, JSON encode/decode helpers, middleware (`Chain`, `CORS`, `Recovery`,
`Logging`), the `X-User-ID` authentication context, and routing
(`RouteRegistrar`, `NewApplicationHandler`).

Each feature package follows the same internal split, mirroring
`internal/users`:

```text
HTTP request
    -> middleware (pkg/httpapi)
    -> handler.go        (parses the request, calls use cases, writes the response)
    -> service.go         (validation and business rules)
    -> store_postgres.go  (parameterized SQL, context-aware)
    -> PostgreSQL
```

| Feature | Package | Model | Business logic | Storage | HTTP |
| --- | --- | --- | --- | --- | --- |
| Users & skills | `internal/users` | `model.go` | `service.go` | `store_postgres.go` | `handler.go` |
| Services | `internal/services` | `model.go` | `service.go` | `store_postgres.go` | `handler.go` |
| Reviews | `internal/reviews` | `model.go` | `service.go` | `store_postgres.go` | `handler.go` |
| Statistics | `internal/stats` | (in `service.go`) | `service.go` | `store_postgres.go` | `handler.go` |
| Exchanges | `internal/exchanges` | `model.go` | `service.go` | `store_postgres.go` | `handler.go` |
| Credits | `internal/credits` | `model.go` | `service.go` | `store_postgres.go` | (library, no HTTP) |

### Cross-feature dependencies

To avoid import cycles, every cross-feature dependency is a small interface
declared on the consumer side (the package that needs the data), satisfied
implicitly by the producer's concrete type — the same "duck typing" idiom
from the course, just crossing a package boundary instead of staying in one
file:

- `services.SkillChecker` (needs only `UserHasSkill(ctx, userID, name) (bool, error)`)
  is satisfied by `*users.Service` without `internal/services` importing
  `internal/users`.
- `stats.UserExistenceChecker` (`UserExists(ctx, userID) (bool, error)`) is
  satisfied by `*users.Service` the same way.
- `reviews.ServiceExistenceChecker` (`Get(ctx, serviceID) (services.Service, error)`)
  is satisfied by `*services.UseCases`; `internal/reviews` imports
  `internal/services` for the `Service` type only (one-directional, no cycle).
- `reviews.ExchangeLookup` and `stats.ExchangeStatsProvider` are satisfied by
  `*exchanges.UseCases` (`internal/exchanges`), so the reviews and statistics
  features consume real exchange data — see
  [Exchanges and the credit ledger](#exchanges-and-the-credit-ledger-person-3).
- `exchanges.ServiceLookup` (`Get(ctx, id) (services.Service, error)`) is
  satisfied by `*services.UseCases`, and `exchanges.RequesterChecker`
  (`UserExists`) by `*users.Service`, both declared on the consumer side.

Only `cmd/server/main.go` imports every feature package to wire them
together; no feature package imports another feature's concrete types except
where noted above.

Useful integration contracts for the other team members:

- Implement `httpapi.RouteRegistrar` and pass the handler to
  `httpapi.NewApplicationHandler` to add exchange routes.
- Call `httpapi.AuthenticatedUserID(ctx)` to read the user established by the
  auth middleware.
- Call `users.Service.UserExists` and `users.Service.UserHasSkill` for
  cross-feature validation.
- Call `services.UseCases.Get` to load a service, its owner, active status,
  and credit cost when validating an exchange request.
- `internal/exchanges` implements `reviews.ExchangeLookup` and
  `stats.ExchangeStatsProvider` on `*exchanges.UseCases`, wired in
  `cmd/server/main.go`.
- Reuse `httpapi.WriteJSON`, `httpapi.DecodeJSON`, `httpapi.WriteAPIError`,
  `httpapi.PathID`, `httpapi.RequireAuthenticatedUser`, and the sentinel
  errors for consistent HTTP responses.
- The welcome-credit entry is stored in `credit_transactions` with type
  `earn` and a `NULL` exchange ID. Person 3 can extend this journal for
  exchange debits, earnings, and refunds.

Detailed assignments for the remaining features are in
`WORK_PERSON_2_AND_3.md`.
