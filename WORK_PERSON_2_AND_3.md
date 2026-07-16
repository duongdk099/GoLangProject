# BarterSwap - Work for Person 2 and Person 3

This document defines the work owned by **Person 2** and **Person 3**.

**Person 1 owns the common infrastructure, database connection, HTTP helpers,
middlewares, users, and skills.** Coordinate shared types and function signatures
with Person 1 before implementing handlers.

## Rules that apply to both people

- Use Go and a single Go package. Files may be separated, but do not create
  internal sub-packages.
- Use `net/http`, `encoding/json`, `context`, and `database/sql` from the standard
  library.
- The selected SQL driver is the only allowed external dependency.
- Do not use an ORM, Gin, Echo, Chi, or a mutex.
- Read the authenticated user from the `X-User-ID` header through the helper or
  middleware supplied by Person 1.
- HTTP handlers must only parse the request, call business logic, and write the
  response. Put validations and business rules in service functions.
- Put SQL operations in store functions and pass `context.Context` to database
  calls.
- Return consistent JSON errors and correct HTTP status codes.
- Write table-driven unit tests and API tests with `httptest` for your own work.
- Run `gofmt`, `go vet`, and `go test -cover` before merging.

## Person 2 - Services, reviews, and statistics

### Suggested files

```text
services_model.go
services_handler.go
services_service.go
services_store.go
services_test.go
services_handler_test.go

reviews_model.go
reviews_handler.go
reviews_service.go
reviews_store.go
reviews_test.go

stats_handler.go
stats_service.go
stats_store.go
stats_test.go
```

The names are suggestions. Keep the handler, business logic, and storage
responsibilities separated even if different names are chosen.

### A. Service advertisements

Implement these endpoints:

| Method | Path | Result |
| --- | --- | --- |
| `GET` | `/api/services` | List services and apply optional filters |
| `POST` | `/api/services` | Create a service advertisement |
| `GET` | `/api/services/{id}` | Return one service |
| `PUT` | `/api/services/{id}` | Update the authenticated user's service |
| `DELETE` | `/api/services/{id}` | Delete the authenticated user's service |

Supported query parameters for `GET /api/services`:

- `categorie`: exact category filter.
- `ville`: city filter.
- `search`: server-side text search in relevant service text fields.

All filtering must happen in the server/database, not in a client.

Use the following response model from the subject:

```go
type Service struct {
	ID           int    `json:"id"`
	ProviderID   int    `json:"provider_id"`
	Titre        string `json:"titre"`
	Description  string `json:"description,omitempty"`
	Categorie    string `json:"categorie"`
	DureeMinutes int    `json:"duree_minutes"`
	Credits      int    `json:"credits"`
	Ville        string `json:"ville,omitempty"`
	Actif        bool   `json:"actif"`
	CreatedAt    string `json:"created_at"`
}
```

Allowed categories:

```text
Informatique, Jardinage, Bricolage, Cuisine, Musique, Langues, Sport,
Tutorat, Demenagement, Photographie, Animalier, Couture, Autre
```

Required business rules:

- Get `ProviderID` from the authenticated user; do not trust a provider ID sent
  by the client.
- A user may only publish a service related to one of their skills.
- Only the service owner may update or delete it.
- Reject an empty title, an invalid category, non-positive duration, or
  non-positive credit cost.
- Return `404` when the requested service does not exist.
- Use parameterized SQL queries. Never build filters by concatenating raw user
  input into SQL.

### B. Reviews

Implement these endpoints:

| Method | Path | Result |
| --- | --- | --- |
| `POST` | `/api/exchanges/{id}/review` | Create a review for a completed exchange |
| `GET` | `/api/users/{id}/reviews` | List reviews received by a user |
| `GET` | `/api/services/{id}/reviews` | List reviews associated with a service |

Use this model:

```go
type Review struct {
	ID          int    `json:"id"`
	ExchangeID  int    `json:"exchange_id"`
	AuthorID    int    `json:"author_id"`
	TargetID    int    `json:"target_id"`
	Note        int    `json:"note"`
	Commentaire string `json:"commentaire,omitempty"`
	CreatedAt   string `json:"created_at"`
}
```

Required business rules:

- Only the requester or service owner involved in the exchange may review it.
- The exchange must have status `completed`.
- The target is the other participant; do not trust `AuthorID` or `TargetID`
  values supplied by the client.
- A note must be from 1 to 5.
- One author may create only one review per exchange.
- A review cannot be updated or deleted.
- Enforce the duplicate rule in business logic and with a database unique
  constraint where possible.

### C. User statistics

Implement:

| Method | Path | Result |
| --- | --- | --- |
| `GET` | `/api/users/{id}/stats` | Return the user's aggregated statistics |

The response must contain:

```go
type UserStats struct {
	UserID            int     `json:"user_id"`
	ServicesActifs    int     `json:"services_actifs"`
	EchangesCompletes int     `json:"echanges_completes"`
	CreditBalance     int     `json:"credit_balance"`
	NoteMoyenne       float64 `json:"note_moyenne"`
	NbAvis            int     `json:"nb_avis"`
	TotalGagne        int     `json:"total_gagne"`
	TotalDepense      int     `json:"total_depense"`
}
```

Calculate every value from database records. Handle a user with no reviews or
completed exchanges without returning SQL `NULL` conversion errors.

### Dependencies to request from the team

From Person 1:

- Authenticated user ID helper/middleware.
- Shared JSON and error response helpers.
- A function to verify that a user exists.
- A function or query contract to check whether a user has a skill.
- Registration of the service, review, and statistics routes.

From Person 3:

- A stable way to retrieve an exchange with its service, requester, owner, and
  status for review validation.
- Final definitions of exchange statuses and credit transaction types.
- Credit ledger queries needed for balance, total earned, and total spent.

### Minimum tests for Person 2

- Create a valid service and receive `201`.
- Reject a service whose title is empty.
- Reject a service with an invalid category.
- Reject a service when the provider does not have the required skill.
- Prevent another user from updating or deleting a service.
- Filter services by category and city.
- Search services using the `search` query parameter.
- Reject a review for a non-completed exchange.
- Reject a note outside the 1-5 range.
- Reject a second review from the same author for the same exchange.
- Return correct reviews for a user and for a service.
- Return coherent statistics, including the zero-data case.

## Person 3 - Exchanges and credit transactions

This is the most transaction-sensitive part of the application. Status changes
and credit updates must be executed atomically with SQL transactions.

### Suggested files

```text
exchanges_model.go
exchanges_handler.go
exchanges_service.go
exchanges_store.go
exchanges_test.go
exchanges_handler_test.go

credits_model.go
credits_service.go
credits_store.go
credits_test.go
```

### A. Exchange endpoints

Implement:

| Method | Path | Result |
| --- | --- | --- |
| `POST` | `/api/exchanges` | Create an exchange request |
| `GET` | `/api/exchanges` | List exchanges requested or received by the authenticated user |
| `GET` | `/api/exchanges/{id}` | Return one exchange |
| `PUT` | `/api/exchanges/{id}/accept` | Accept a pending request |
| `PUT` | `/api/exchanges/{id}/reject` | Reject a pending request |
| `PUT` | `/api/exchanges/{id}/complete` | Mark an accepted exchange as completed |
| `PUT` | `/api/exchanges/{id}/cancel` | Cancel an exchange |

`GET /api/exchanges` must support an optional `status` query parameter.

Use this model:

```go
type Exchange struct {
	ID          int    `json:"id"`
	ServiceID   int    `json:"service_id"`
	RequesterID int    `json:"requester_id"`
	OwnerID     int    `json:"owner_id"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}
```

Supported lifecycle:

```text
pending -> accepted -> completed
   |           |
   v           v
rejected    cancelled
```

Required business rules:

- The requester ID comes from authentication.
- Load the service to determine its owner and credit cost; do not trust an owner
  ID or price sent by the client.
- A user cannot request their own service.
- A service can have only one `pending` or `accepted` exchange at a time.
- The requester must have enough credits when requesting an exchange. Check the
  balance again inside the acceptance transaction to avoid stale decisions.
- Only the service owner may accept or reject a pending request.
- Only authenticated participants may see or cancel an exchange.
- Agree with the team which participant may call `complete`, then enforce that
  decision consistently and document it in the README.
- Reject invalid status transitions. Repeating a successful transition must not
  transfer credits a second time.
- Return `409 Conflict` when the service is already reserved.
- Execute conflict checks, state changes, and credit operations safely through
  the database. Do not use a Go mutex.

### B. Credit journal

Use a transaction journal rather than treating a mutable balance column as the
only source of truth:

```go
type CreditTransaction struct {
	ID         int    `json:"id"`
	UserID     int    `json:"user_id"`
	ExchangeID int    `json:"exchange_id"`
	Montant    int    `json:"montant"`
	Type       string `json:"type"`
	CreatedAt  string `json:"created_at"`
}
```

Credit behavior:

- On account creation, Person 1 must be able to record the 10 welcome credits
  through the journal contract you provide.
- On `accept`, debit/block the service cost from the requester.
- On `complete`, credit that amount to the service owner.
- On cancellation after acceptance, refund the requester.
- Rejecting a still-pending exchange must not create a refund because no credit
  has been blocked yet.
- Every status and credit change belonging to one action must commit or roll back
  together in one SQL transaction.
- Make the journal entries traceable to the relevant exchange. Decide with
  Person 1 how a welcome-credit entry represents the absence of an exchange
  (`NULL` exchange ID is preferable if the schema allows it).
- Prevent duplicate debit, earning, or refund entries for the same exchange and
  operation, ideally with database constraints in addition to service checks.

### Database/concurrency expectations

- Use `database/sql` transactions for `accept`, `complete`, and cancellation
  after acceptance.
- Lock or conditionally update the relevant rows according to the selected
  database.
- Add an appropriate database constraint/index or equivalent transactional
  protection so two simultaneous requests cannot reserve one service.
- Always check affected row counts when using a conditional update.
- Do not hold a transaction open while performing unrelated work.

### Dependencies to request from the team

From Person 1:

- Authenticated user ID helper/middleware.
- Shared JSON and error response helpers.
- A function to verify that a requester exists.
- Route registration and database transaction conventions.
- User creation must call the agreed welcome-credit journal function.

From Person 2:

- A stable function/query contract for loading a service, including its owner,
  active status, and credit cost.

Provide to Person 2:

- Exchange lookup needed to validate reviews.
- Credit balance, earned-total, and spent-total queries needed for statistics.

### Minimum tests for Person 3

- Create a valid exchange request.
- Reject a request for the user's own service with `400`.
- Reject a request when the requester has insufficient credits with `400`.
- Return `409` when the service already has a pending or accepted exchange.
- Prevent a non-owner from accepting or rejecting a request.
- Accept an exchange and debit/block the requester credits exactly once.
- Reject invalid lifecycle transitions.
- Complete an accepted exchange and credit the owner exactly once.
- Cancel an accepted exchange and refund the requester exactly once.
- Reject a pending exchange without creating a refund.
- List only exchanges requested or received by the authenticated user.
- Filter the list by status.
- Roll back both the status and journal operation when any SQL step fails.

## Integration plan

### Step 1 - Agree on contracts before coding

The three people should agree on:

- SQL database choice and timestamp format.
- Shared model names and JSON field names.
- Store/service function signatures used across features.
- Sentinel errors such as not found, invalid input, forbidden, insufficient
  credit, invalid transition, and reservation conflict.
- The mapping from those errors to HTTP status codes.

### Step 2 - Work on separate branches

Suggested branches:

```text
feature/services-reviews-stats
feature/exchanges-credits
```

Avoid editing Person 1's infrastructure files without coordination. Give Person
1 any required routes, schema changes, or shared contracts in small commits.

### Step 3 - Integrate in this order

1. Common server, database, users, skills, and authentication from Person 1.
2. Service creation and lookup from Person 2.
3. Exchange lifecycle and credits from Person 3.
4. Reviews and statistics from Person 2.
5. End-to-end tests and README examples.

### Step 4 - Required end-to-end scenario

Before considering the work complete, demonstrate this sequence:

1. Create two users and give the provider a skill.
2. Publish a service for that skill.
3. Request the service from the second user.
4. Accept and complete the exchange.
5. Verify both credit balances and journal entries.
6. Submit a review.
7. Verify user and service reviews.
8. Verify the user statistics.
9. Demonstrate at least one `400`, one `403` or equivalent authorization error,
   one `404`, and the `409` reservation conflict.

## Definition of done

A person's part is complete only when:

- All assigned endpoints work through HTTP, not only through direct function
  calls.
- Business logic is outside HTTP handlers.
- SQL calls use contexts and parameterized queries.
- Errors produce consistent JSON and suitable HTTP codes.
- Tests cover successful paths and important failures.
- The complete repository reaches at least 60% test coverage.
- `gofmt -w .`, `go vet ./...`, and `go test -v -cover ./...` succeed.
- Their endpoints and example `curl` commands are documented for the final
  README and demonstration.
