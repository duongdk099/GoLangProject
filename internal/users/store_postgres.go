package users

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

func (s *PostgresStore) Create(ctx context.Context, params CreateUserParams) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("begin create user transaction: %w", err)
	}
	defer tx.Rollback()

	var user User
	var createdAt time.Time
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (pseudo, bio, ville)
		VALUES ($1, $2, $3)
		RETURNING id, pseudo, bio, ville, created_at
	`, params.Pseudo, params.Bio, params.Ville).Scan(
		&user.ID, &user.Pseudo, &user.Bio, &user.Ville, &createdAt,
	)
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO credit_transactions (user_id, exchange_id, montant, type)
		VALUES ($1, NULL, $2, 'earn')
	`, user.ID, WelcomeCredits)
	if err != nil {
		return User{}, fmt.Errorf("insert welcome credits: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("commit create user transaction: %w", err)
	}

	user.CreditBalance = WelcomeCredits
	user.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	user.Skills = []Skill{}
	return user, nil
}

func (s *PostgresStore) GetByID(ctx context.Context, userID int) (User, error) {
	var user User
	var createdAt time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.pseudo, u.bio, u.ville, u.created_at,
		       COALESCE(SUM(ct.montant), 0)::INTEGER
		FROM users u
		LEFT JOIN credit_transactions ct ON ct.user_id = u.id
		WHERE u.id = $1
		GROUP BY u.id
	`, userID).Scan(
		&user.ID, &user.Pseudo, &user.Bio, &user.Ville, &createdAt, &user.CreditBalance,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, httpapi.ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user %d: %w", userID, err)
	}
	user.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return user, nil
}

func (s *PostgresStore) Update(ctx context.Context, userID int, params UpdateUserParams) (User, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET pseudo = $2, bio = $3, ville = $4
		WHERE id = $1
	`, userID, params.Pseudo, params.Bio, params.Ville)
	if err != nil {
		return User{}, fmt.Errorf("update user %d: %w", userID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return User{}, fmt.Errorf("check updated user %d: %w", userID, err)
	}
	if rows == 0 {
		return User{}, httpapi.ErrNotFound
	}
	return s.GetByID(ctx, userID)
}

func (s *PostgresStore) ListSkills(ctx context.Context, userID int) ([]Skill, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT nom, niveau
		FROM skills
		WHERE user_id = $1
		ORDER BY LOWER(nom), nom
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list skills for user %d: %w", userID, err)
	}
	defer rows.Close()

	skills := make([]Skill, 0)
	for rows.Next() {
		var skill Skill
		if err := rows.Scan(&skill.Nom, &skill.Niveau); err != nil {
			return nil, fmt.Errorf("scan skill for user %d: %w", userID, err)
		}
		skills = append(skills, skill)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate skills for user %d: %w", userID, err)
	}
	return skills, nil
}

func (s *PostgresStore) ReplaceSkills(ctx context.Context, userID int, skills []Skill) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace skills transaction: %w", err)
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return httpapi.ErrNotFound
		}
		return fmt.Errorf("lock user %d: %w", userID, err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM skills WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear skills for user %d: %w", userID, err)
	}
	for _, skill := range skills {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO skills (user_id, nom, niveau)
			VALUES ($1, $2, $3)
		`, userID, skill.Nom, skill.Niveau); err != nil {
			return fmt.Errorf("insert skill %q for user %d: %w", skill.Nom, userID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace skills transaction: %w", err)
	}
	return nil
}

func (s *PostgresStore) Exists(ctx context.Context, userID int) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check user %d: %w", userID, err)
	}
	return exists, nil
}

func (s *PostgresStore) HasSkill(ctx context.Context, userID int, name string) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM skills
			WHERE user_id = $1 AND LOWER(nom) = LOWER($2)
		)
	`, userID, name).Scan(&exists); err != nil {
		return false, fmt.Errorf("check skill %q for user %d: %w", name, userID, err)
	}
	return exists, nil
}
