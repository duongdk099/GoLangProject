package exchanges

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"barterswap/internal/credits"
	"barterswap/pkg/httpapi"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// lockedExchange is an exchange read FOR UPDATE inside a transition, carrying
// the internal credit cost alongside the public fields.
type lockedExchange struct {
	Exchange
	cost int
}

func (s *PostgresStore) Create(ctx context.Context, params CreateParams) (Exchange, error) {
	var exchange Exchange
	var createdAt, updatedAt time.Time
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO exchanges (service_id, requester_id, owner_id, credits_cost)
		VALUES ($1, $2, $3, $4)
		RETURNING id, service_id, requester_id, owner_id, status, created_at, updated_at
	`, params.ServiceID, params.RequesterID, params.OwnerID, params.Cost).Scan(
		&exchange.ID, &exchange.ServiceID, &exchange.RequesterID, &exchange.OwnerID,
		&exchange.Status, &createdAt, &updatedAt,
	)
	if isUniqueViolation(err) {
		return Exchange{}, fmt.Errorf("%w: this service already has a pending or accepted exchange", httpapi.ErrConflict)
	}
	if err != nil {
		return Exchange{}, fmt.Errorf("insert exchange: %w", err)
	}
	exchange.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	exchange.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return exchange, nil
}

func (s *PostgresStore) GetByID(ctx context.Context, exchangeID int) (Exchange, error) {
	var exchange Exchange
	var createdAt, updatedAt time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT id, service_id, requester_id, owner_id, status, created_at, updated_at
		FROM exchanges
		WHERE id = $1
	`, exchangeID).Scan(
		&exchange.ID, &exchange.ServiceID, &exchange.RequesterID, &exchange.OwnerID,
		&exchange.Status, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Exchange{}, httpapi.ErrNotFound
	}
	if err != nil {
		return Exchange{}, fmt.Errorf("get exchange %d: %w", exchangeID, err)
	}
	exchange.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	exchange.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return exchange, nil
}

func (s *PostgresStore) List(ctx context.Context, filter Filter) ([]Exchange, error) {
	args := []any{filter.UserID}
	query := `
		SELECT id, service_id, requester_id, owner_id, status, created_at, updated_at
		FROM exchanges
		WHERE (requester_id = $1 OR owner_id = $1)
	`
	if filter.Status != "" {
		args = append(args, filter.Status)
		query += fmt.Sprintf(" AND status = $%d", len(args))
	}
	query += " ORDER BY created_at DESC, id DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list exchanges: %w", err)
	}
	defer rows.Close()

	exchanges := make([]Exchange, 0)
	for rows.Next() {
		var exchange Exchange
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&exchange.ID, &exchange.ServiceID, &exchange.RequesterID, &exchange.OwnerID,
			&exchange.Status, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan exchange: %w", err)
		}
		exchange.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		exchange.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		exchanges = append(exchanges, exchange)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate exchanges: %w", err)
	}
	return exchanges, nil
}

func (s *PostgresStore) CountCompleted(ctx context.Context, userID int) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM exchanges
		WHERE status = 'completed' AND (requester_id = $1 OR owner_id = $1)
	`, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count completed exchanges for user %d: %w", userID, err)
	}
	return count, nil
}

func (s *PostgresStore) Balance(ctx context.Context, userID int) (int, error) {
	return credits.Balance(ctx, s.db, userID)
}

func (s *PostgresStore) Accept(ctx context.Context, exchangeID, ownerID int) (Exchange, error) {
	return s.transition(ctx, exchangeID, transition{
		requiredStatus: StatusPending,
		nextStatus:     StatusAccepted,
		authorize: func(ex lockedExchange) error {
			if ex.OwnerID != ownerID {
				return fmt.Errorf("%w: only the service owner may accept this request", httpapi.ErrForbidden)
			}
			return nil
		},
		credit: func(ctx context.Context, tx *sql.Tx, ex lockedExchange) error {
			// Re-check the balance under the row lock before blocking credits.
			balance, err := credits.Balance(ctx, tx, ex.RequesterID)
			if err != nil {
				return err
			}
			if balance < ex.cost {
				return fmt.Errorf("%w: the requester no longer has enough credits", httpapi.ErrValidation)
			}
			return credits.Record(ctx, tx, credits.Entry{
				UserID: ex.RequesterID, ExchangeID: ex.ID, Amount: ex.cost, Type: credits.TypeSpend,
			})
		},
	})
}

func (s *PostgresStore) Reject(ctx context.Context, exchangeID, ownerID int) (Exchange, error) {
	return s.transition(ctx, exchangeID, transition{
		requiredStatus: StatusPending,
		nextStatus:     StatusRejected,
		authorize: func(ex lockedExchange) error {
			if ex.OwnerID != ownerID {
				return fmt.Errorf("%w: only the service owner may reject this request", httpapi.ErrForbidden)
			}
			return nil
		},
		// No credit was blocked on a pending request, so rejection moves no credit.
	})
}

func (s *PostgresStore) Complete(ctx context.Context, exchangeID, requesterID int) (Exchange, error) {
	return s.transition(ctx, exchangeID, transition{
		requiredStatus: StatusAccepted,
		nextStatus:     StatusCompleted,
		authorize: func(ex lockedExchange) error {
			if ex.RequesterID != requesterID {
				return fmt.Errorf("%w: only the requester may confirm completion", httpapi.ErrForbidden)
			}
			return nil
		},
		credit: func(ctx context.Context, tx *sql.Tx, ex lockedExchange) error {
			return credits.Record(ctx, tx, credits.Entry{
				UserID: ex.OwnerID, ExchangeID: ex.ID, Amount: ex.cost, Type: credits.TypeEarn,
			})
		},
	})
}

// Cancel is not a simple single-source transition: it is valid from both
// pending and accepted, and only the accepted case refunds the requester, so
// it does not reuse the transition helper.
func (s *PostgresStore) Cancel(ctx context.Context, exchangeID, actorID int) (Exchange, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Exchange{}, fmt.Errorf("begin cancel transaction: %w", err)
	}
	defer tx.Rollback()

	locked, err := lockExchange(ctx, tx, exchangeID)
	if err != nil {
		return Exchange{}, err
	}
	if actorID != locked.RequesterID && actorID != locked.OwnerID {
		return Exchange{}, fmt.Errorf("%w: only a participant may cancel this exchange", httpapi.ErrForbidden)
	}

	switch locked.Status {
	case StatusPending:
		// Nothing was blocked, so nothing is refunded.
	case StatusAccepted:
		if err := credits.Record(ctx, tx, credits.Entry{
			UserID: locked.RequesterID, ExchangeID: locked.ID, Amount: locked.cost, Type: credits.TypeRefund,
		}); err != nil {
			return Exchange{}, err
		}
	default:
		return Exchange{}, fmt.Errorf("%w: a %s exchange cannot be cancelled", httpapi.ErrValidation, locked.Status)
	}

	updated, err := commitStatus(ctx, tx, exchangeID, StatusCancelled, locked.Status)
	if err != nil {
		return Exchange{}, err
	}
	if err := tx.Commit(); err != nil {
		return Exchange{}, fmt.Errorf("commit cancel transaction: %w", err)
	}
	return updated, nil
}

// transition describes an atomic move from exactly one status to another,
// with an optional credit movement performed inside the same transaction.
type transition struct {
	requiredStatus string
	nextStatus     string
	authorize      func(lockedExchange) error
	credit         func(ctx context.Context, tx *sql.Tx, ex lockedExchange) error
}

func (s *PostgresStore) transition(ctx context.Context, exchangeID int, t transition) (Exchange, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Exchange{}, fmt.Errorf("begin %s transition: %w", t.nextStatus, err)
	}
	defer tx.Rollback()

	locked, err := lockExchange(ctx, tx, exchangeID)
	if err != nil {
		return Exchange{}, err
	}
	if err := t.authorize(locked); err != nil {
		return Exchange{}, err
	}
	if locked.Status != t.requiredStatus {
		return Exchange{}, fmt.Errorf("%w: a %s exchange cannot become %s", httpapi.ErrValidation, locked.Status, t.nextStatus)
	}
	if t.credit != nil {
		if err := t.credit(ctx, tx, locked); err != nil {
			return Exchange{}, err
		}
	}

	updated, err := commitStatus(ctx, tx, exchangeID, t.nextStatus, t.requiredStatus)
	if err != nil {
		return Exchange{}, err
	}
	if err := tx.Commit(); err != nil {
		return Exchange{}, fmt.Errorf("commit %s transition: %w", t.nextStatus, err)
	}
	return updated, nil
}

// lockExchange reads an exchange FOR UPDATE, serializing concurrent transitions
// on the same row so credits and status stay consistent.
func lockExchange(ctx context.Context, tx *sql.Tx, exchangeID int) (lockedExchange, error) {
	var locked lockedExchange
	var createdAt, updatedAt time.Time
	err := tx.QueryRowContext(ctx, `
		SELECT id, service_id, requester_id, owner_id, status, credits_cost, created_at, updated_at
		FROM exchanges
		WHERE id = $1
		FOR UPDATE
	`, exchangeID).Scan(
		&locked.ID, &locked.ServiceID, &locked.RequesterID, &locked.OwnerID,
		&locked.Status, &locked.cost, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return lockedExchange{}, httpapi.ErrNotFound
	}
	if err != nil {
		return lockedExchange{}, fmt.Errorf("lock exchange %d: %w", exchangeID, err)
	}
	locked.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	locked.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return locked, nil
}

// commitStatus applies the status change guarded by the expected current
// status. The guard plus RowsAffected check means that if the row moved out of
// the expected status between the lock and the update (which the row lock
// prevents, but the guard keeps the invariant explicit), no phantom update is
// reported as success.
func commitStatus(ctx context.Context, tx *sql.Tx, exchangeID int, nextStatus, fromStatus string) (Exchange, error) {
	var exchange Exchange
	var createdAt, updatedAt time.Time
	err := tx.QueryRowContext(ctx, `
		UPDATE exchanges
		SET status = $2, updated_at = NOW()
		WHERE id = $1 AND status = $3
		RETURNING id, service_id, requester_id, owner_id, status, created_at, updated_at
	`, exchangeID, nextStatus, fromStatus).Scan(
		&exchange.ID, &exchange.ServiceID, &exchange.RequesterID, &exchange.OwnerID,
		&exchange.Status, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Exchange{}, httpapi.ErrConflict
	}
	if err != nil {
		return Exchange{}, fmt.Errorf("update exchange %d to %s: %w", exchangeID, nextStatus, err)
	}
	exchange.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	exchange.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return exchange, nil
}

// isUniqueViolation reports whether err is a PostgreSQL unique constraint
// violation (SQLSTATE 23505), mirroring the helper in internal/reviews and
// internal/credits.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pqErr interface{ SQLState() string }
	if errors.As(err, &pqErr) {
		return pqErr.SQLState() == "23505"
	}
	return false
}
