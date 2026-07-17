package credits

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"barterswap/pkg/httpapi"
)

type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func Record(ctx context.Context, exec Execer, entry Entry) error {
	if err := Validate(entry); err != nil {
		return err
	}

	var exchangeID any
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
