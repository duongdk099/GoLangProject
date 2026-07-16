package stats

import (
	"context"
	"database/sql"
	"fmt"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) CountActiveServices(ctx context.Context, providerID int) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM services WHERE provider_id = $1 AND actif = TRUE
	`, providerID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active services for user %d: %w", providerID, err)
	}
	return count, nil
}

func (s *PostgresStore) CreditBalance(ctx context.Context, userID int) (int, error) {
	var balance int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(montant), 0)::INTEGER
		FROM credit_transactions
		WHERE user_id = $1
	`, userID).Scan(&balance); err != nil {
		return 0, fmt.Errorf("credit balance for user %d: %w", userID, err)
	}
	return balance, nil
}

func (s *PostgresStore) CreditTotals(ctx context.Context, userID int) (earned, spent int, err error) {
	err = s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(montant) FILTER (WHERE type = 'earn'), 0)::INTEGER,
			COALESCE(-SUM(montant) FILTER (WHERE type = 'spend'), 0)::INTEGER
		FROM credit_transactions
		WHERE user_id = $1
	`, userID).Scan(&earned, &spent)
	if err != nil {
		return 0, 0, fmt.Errorf("credit totals for user %d: %w", userID, err)
	}
	return earned, spent, nil
}

func (s *PostgresStore) ReviewAggregate(ctx context.Context, targetID int) (average float64, count int, err error) {
	err = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(AVG(note), 0)::FLOAT8, COUNT(*)
		FROM reviews
		WHERE target_id = $1
	`, targetID).Scan(&average, &count)
	if err != nil {
		return 0, 0, fmt.Errorf("review aggregate for user %d: %w", targetID, err)
	}
	return average, count, nil
}
