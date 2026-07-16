package reviews

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"barterswap/pkg/httpapi"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) Create(ctx context.Context, params CreateParams) (Review, error) {
	var review Review
	var createdAt time.Time
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO reviews (exchange_id, service_id, author_id, target_id, note, commentaire)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, exchange_id, author_id, target_id, note, commentaire, created_at
	`, params.ExchangeID, params.ServiceID, params.AuthorID, params.TargetID, params.Note, params.Commentaire).Scan(
		&review.ID, &review.ExchangeID, &review.AuthorID, &review.TargetID, &review.Note, &review.Commentaire, &createdAt,
	)
	if isUniqueViolation(err) {
		return Review{}, fmt.Errorf("%w: you already reviewed this exchange", httpapi.ErrValidation)
	}
	if err != nil {
		return Review{}, fmt.Errorf("insert review for exchange %d: %w", params.ExchangeID, err)
	}
	review.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return review, nil
}

func (s *PostgresStore) ExistsForAuthor(ctx context.Context, exchangeID, authorID int) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM reviews WHERE exchange_id = $1 AND author_id = $2
		)
	`, exchangeID, authorID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check existing review for exchange %d: %w", exchangeID, err)
	}
	return exists, nil
}

func (s *PostgresStore) ListByTarget(ctx context.Context, targetID int) ([]Review, error) {
	return s.query(ctx, `
		SELECT id, exchange_id, author_id, target_id, note, commentaire, created_at
		FROM reviews
		WHERE target_id = $1
		ORDER BY created_at DESC, id DESC
	`, targetID)
}

func (s *PostgresStore) ListByService(ctx context.Context, serviceID int) ([]Review, error) {
	return s.query(ctx, `
		SELECT id, exchange_id, author_id, target_id, note, commentaire, created_at
		FROM reviews
		WHERE service_id = $1
		ORDER BY created_at DESC, id DESC
	`, serviceID)
}

func (s *PostgresStore) query(ctx context.Context, query string, arg int) ([]Review, error) {
	rows, err := s.db.QueryContext(ctx, query, arg)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	defer rows.Close()

	reviews := make([]Review, 0)
	for rows.Next() {
		var review Review
		var createdAt time.Time
		if err := rows.Scan(
			&review.ID, &review.ExchangeID, &review.AuthorID, &review.TargetID,
			&review.Note, &review.Commentaire, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		review.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		reviews = append(reviews, review)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reviews: %w", err)
	}
	return reviews, nil
}

// isUniqueViolation reports whether err is a PostgreSQL unique constraint
// violation (SQLSTATE 23505), without importing the driver's error type
// directly so this file only depends on database/sql and fmt/errors.
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
