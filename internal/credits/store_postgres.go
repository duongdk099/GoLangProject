package credits

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"barterswap/pkg/httpapi"
)

// Execer is the subset of *sql.DB and *sql.Tx the journal needs. Accepting it
// lets a caller record a movement standalone (passing the *sql.DB) or inside a
// transaction it already opened (passing the *sql.Tx), so a credit movement
// can commit or roll back together with the status change that caused it.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Record validates and appends one movement to the journal. When two callers
// try to record the same movement for one exchange (same exchange_id and
// type), the unique index makes the second INSERT fail; that is surfaced as a
// conflict so a debit, earning, or refund can never be written twice.
func Record(ctx context.Context, exec Execer, entry Entry) error {
	if err := Validate(entry); err != nil {
		return err
	}

	var exchangeID any // stored as NULL when the movement is not tied to an exchange
	if entry.ExchangeID > 0 {
		exchangeID = entry.ExchangeID
	}

	_, err := exec.ExecContext(ctx, `
		INSERT INTO credit_transactions (user_id, exchange_id, montant, type)
		VALUES ($1, $2, $3, $4)
	`, entry.UserID, exchangeID, signedMontant(entry), entry.Type)
	if isUniqueViolation(err) {
		return fmt.Errorf("%w: this credit movement was already recorded", httpapi.ErrConflict)
	}
	if err != nil {
		return fmt.Errorf("record credit entry: %w", err)
	}
	return nil
}

// Balance returns the sum of a user's journal entries. A user with no entries
// has a balance of zero rather than a NULL conversion error.
func Balance(ctx context.Context, exec Execer, userID int) (int, error) {
	var balance int
	if err := exec.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(montant), 0)::INTEGER
		FROM credit_transactions
		WHERE user_id = $1
	`, userID).Scan(&balance); err != nil {
		return 0, fmt.Errorf("credit balance for user %d: %w", userID, err)
	}
	return balance, nil
}

// isUniqueViolation reports whether err is a PostgreSQL unique constraint
// violation (SQLSTATE 23505), detected through the driver-agnostic SQLState
// interface so this file depends only on database/sql. This mirrors the same
// helper in internal/reviews.
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
