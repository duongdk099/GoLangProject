# Test coverage report

This document records the test-coverage status of every package, grouped by
owner, and explains exactly which statements are **not** covered and why. It is
kept next to the code so anyone can reproduce the numbers.

## How to reproduce

Unit tests only (no database):

```bash
go test ./... -cover
```

Full coverage including the PostgreSQL stores (the DB-backed `store_postgres.go`
files only run with the integration flag). The packages **must** be run serially
with `-p 1`, because every package's integration test calls `database.Migrate`,
and Postgres `CREATE TABLE IF NOT EXISTS` is not safe under concurrent execution
(parallel migrations collide on `pg_type` with `duplicate key ...
pg_type_typname_nsp_index`):

```bash
docker compose up -d postgres
RUN_POSTGRES_INTEGRATION=1 \
TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/barterswap?sslmode=disable' \
go test ./... -p 1 -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1      # total
go tool cover -func=coverage.out | awk '$3!="100.0%"'   # every function below 100%
```

## Summary (with integration tests, `-p 1`)

| Owner | Package | Coverage | Notes |
| --- | --- | ---: | --- |
| Person 1 | `internal/config` | 100.0% | |
| Person 1 | `internal/testutil` | 100.0% | |
| Person 1 | `internal/users` | 82.7% | untested error branches |
| Person 1 | `pkg/httpapi` | 59.1% | several funcs exercised only cross-package (see note) |
| Person 1 | `internal/database` | 0.0% | exercised by every integration test, but not self-credited (see note) |
| Person 2 | `internal/reviews` | 80.9% | untested error **and some logic** branches |
| Person 2 | `internal/services` | 79.9% | untested error **and some logic** branches |
| Person 2 | `internal/stats` | 78.8% | untested error branches |
| **Person 3** | **`internal/credits`** | **100.0%** | |
| **Person 3** | **`internal/exchanges`** | **94.9%** | only defensive DB-fault branches remain (see below) |
| shared | `cmd/server` | 0.0% | `main()` process wiring, not unit-tested |
| — | **total** | **78.8%** | above the 60% definition-of-done target |

Person 3's two packages are the **highest-covered feature area** in the
repository. `credits` is at 100%. `exchanges` is at 94.9% — every handler,
service, and model statement is covered; the only uncovered statements are
defensive database-failure branches in `store_postgres.go`.

## Person 3 — the 12 remaining statements (all in `internal/exchanges/store_postgres.go`)

Every uncovered statement is a `return ... err` guard that only executes when a
database call **inside an already-open transaction fails**, a **commit fails**,
or a **lock-guarded race** is lost. None can be triggered against a real
PostgreSQL driver without injecting a fault into `database/sql` itself, so
covering them would mean adding a mock in place of the real driver — which the
project rules discourage and which the other stores also do not do.

| Function | Uncovered guard(s) | Why unreachable with a real driver |
| --- | --- | --- |
| `List` | `rows.Scan` failure; `rows.Err()` failure | rows returned by a successful query over valid columns do not fail to scan or iterate |
| `Accept` | in-transaction balance re-read failure | the `SELECT SUM(...)` inside the acceptance tx does not fail once the tx is open |
| `Cancel` | refund-write failure; status-write failure; commit failure | the `INSERT`/`UPDATE`/`COMMIT` inside the cancel tx do not fail on a healthy connection |
| `transition` | commit failure | `COMMIT` does not fail on a healthy connection |
| `lockExchange` | non-"not found" lock-read failure | `SELECT ... FOR UPDATE` either finds the row or returns `ErrNoRows` (both covered); other failures need a broken connection mid-tx |
| `commitStatus` | lost-race (`ErrNoRows`) branch; write failure | the row is held `FOR UPDATE`, so the guarded `UPDATE` always matches; the race branch is defense-in-depth that cannot fire while the lock is held |

What **is** covered for these functions: every happy path, every
authorization/validation/not-found/conflict branch, the reservation `409`, the
"exactly once" credit movements, invalid transitions, and — via a deliberately
closed connection pool — the top-level `BeginTx`/`Query`/`Exec` error returns of
`Create`, `GetByID`, `List`, `CountCompleted`, `Balance`, `Accept`, and
`Cancel`.

## Other parts below 100% (Person 1 and Person 2)

These gaps are **pre-existing** and were not introduced by Person 3's work. They
are listed here for completeness so it is clear where the repository's remaining
uncovered code lives.

### Person 1 (`internal/users`, `pkg/httpapi`, `internal/database`, `cmd/server`)

| File | Function | Coverage |
| --- | --- | ---: |
| `internal/users/service.go` | `Update` | 71.4% |
| `internal/users/service.go` | `Skills` | 76.9% |
| `internal/users/service.go` | `Get` | 81.8% |
| `internal/users/service.go` | `SetSkills` | 90.5% |
| `internal/users/handler.go` | `getSkills` | 62.5% |
| `internal/users/handler.go` | `setSkills` | 73.3% |
| `internal/users/handler.go` | `update` | 80.0% |
| `internal/users/store_postgres.go` | `ReplaceSkills` | 70.6% |
| `internal/users/store_postgres.go` | `Create` / `Update` | 77.8% |
| `internal/users/store_postgres.go` | `ListSkills` | 76.9% |
| `internal/users/store_postgres.go` | `Exists` / `HasSkill` | 75.0% |
| `internal/users/store_postgres.go` | `GetByID` | 88.9% |
| `pkg/httpapi/json.go` | `WriteAPIError` | 40.0% |
| `pkg/httpapi/json.go` | `DecodeJSON`, `WriteBadRequest`, `PathID` | 0.0% (see note) |
| `pkg/httpapi/auth.go` | `RequireAuthenticatedUser` | 0.0% (see note) |
| `pkg/httpapi/auth.go` | `Authentication` | 81.8% |
| `pkg/httpapi/router.go` | `NewRouter` | 87.5% |
| `pkg/httpapi/middleware.go` | `Logging` | 87.5% |
| `internal/database/database.go` | `Open`, `Migrate` | 0.0% (see note) |
| `cmd/server/main.go` | `main` | 0.0% |

### Person 2 (`internal/services`, `internal/reviews`, `internal/stats`)

Some of these are **logic** branches, not just database-failure guards — e.g.
`services.Get` (66.7%), the services `delete` handler (60.0%), and
`reviews.ListForUser` (62.5%).

| File | Function | Coverage |
| --- | --- | ---: |
| `internal/services/service.go` | `Get` | 66.7% |
| `internal/services/service.go` | `Update` | 73.3% |
| `internal/services/service.go` | `validateFields` | 80.0% |
| `internal/services/service.go` | `List` | 81.8% |
| `internal/services/service.go` | `Delete` | 83.3% |
| `internal/services/service.go` | `Create` | 88.9% |
| `internal/services/handler.go` | `delete` | 60.0% |
| `internal/services/handler.go` | `update` | 73.3% |
| `internal/services/handler.go` | `create` | 84.6% |
| `internal/services/handler.go` | `get` | 87.5% |
| `internal/services/store_postgres.go` | `Update` | 66.7% |
| `internal/services/store_postgres.go` | `List` | 75.0% |
| `internal/services/store_postgres.go` | `Delete` | 77.8% |
| `internal/services/store_postgres.go` | `Create` | 85.7% |
| `internal/services/store_postgres.go` | `GetByID` | 88.9% |
| `internal/reviews/service.go` | `ListForUser` | 62.5% |
| `internal/reviews/service.go` | `ListForService` | 75.0% |
| `internal/reviews/service.go` | `Create` | 85.7% |
| `internal/reviews/handler.go` | `listForUser` | 62.5% |
| `internal/reviews/handler.go` | `create` | 80.0% |
| `internal/reviews/handler.go` | `listForService` | 87.5% |
| `internal/reviews/store_postgres.go` | `ExistsForAuthor` | 75.0% |
| `internal/reviews/store_postgres.go` | `query` | 80.0% |
| `internal/reviews/store_postgres.go` | `isUniqueViolation` | 83.3% |
| `internal/reviews/store_postgres.go` | `Create` | 88.9% |
| `internal/stats/service.go` | `Get` | 75.0% |
| `internal/stats/store_postgres.go` | `CountActiveServices`, `CreditBalance`, `CreditTotals`, `ReviewAggregate` | 75.0% |
| `internal/stats/handler.go` | `get` | 87.5% |

## Notes

- **`0.0%` in `pkg/httpapi` and `internal/database`.** `DecodeJSON`, `PathID`,
  `WriteBadRequest`, `RequireAuthenticatedUser`, `database.Open`, and
  `database.Migrate` *do* run during other packages' HTTP and integration tests,
  but Go's per-package coverage only credits statements to the test binary of
  the package that owns them. Measuring with `-coverpkg=./...` attributes these
  cross-package calls and raises their numbers; the per-package view above does
  not.
- **`gofmt` and line endings.** The repository is checked out with
  `core.autocrlf=true`, so committed `.go` files have CRLF line terminators and
  `gofmt -l` lists them. This is a line-ending artifact, not a formatting
  problem: stripping CR (`tr -d '\r' < file | gofmt -d`) yields no diff for any
  source file in this project.
