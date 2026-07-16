package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"barterswap/pkg/httpapi"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) Create(ctx context.Context, params CreateParams) (Service, error) {
	var service Service
	var createdAt time.Time
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO services (provider_id, titre, description, categorie, duree_minutes, credits, ville)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, provider_id, titre, description, categorie, duree_minutes, credits, ville, actif, created_at
	`, params.ProviderID, params.Titre, params.Description, params.Categorie, params.DureeMinutes, params.Credits, params.Ville).Scan(
		&service.ID, &service.ProviderID, &service.Titre, &service.Description, &service.Categorie,
		&service.DureeMinutes, &service.Credits, &service.Ville, &service.Actif, &createdAt,
	)
	if err != nil {
		return Service{}, fmt.Errorf("insert service: %w", err)
	}
	service.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return service, nil
}

func (s *PostgresStore) GetByID(ctx context.Context, serviceID int) (Service, error) {
	var service Service
	var createdAt time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT id, provider_id, titre, description, categorie, duree_minutes, credits, ville, actif, created_at
		FROM services
		WHERE id = $1
	`, serviceID).Scan(
		&service.ID, &service.ProviderID, &service.Titre, &service.Description, &service.Categorie,
		&service.DureeMinutes, &service.Credits, &service.Ville, &service.Actif, &createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Service{}, httpapi.ErrNotFound
	}
	if err != nil {
		return Service{}, fmt.Errorf("get service %d: %w", serviceID, err)
	}
	service.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return service, nil
}

func (s *PostgresStore) Update(ctx context.Context, serviceID int, params UpdateParams) (Service, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE services
		SET titre = $2, description = $3, categorie = $4, duree_minutes = $5,
		    credits = $6, ville = $7, actif = $8
		WHERE id = $1
	`, serviceID, params.Titre, params.Description, params.Categorie, params.DureeMinutes,
		params.Credits, params.Ville, params.Actif)
	if err != nil {
		return Service{}, fmt.Errorf("update service %d: %w", serviceID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return Service{}, fmt.Errorf("check updated service %d: %w", serviceID, err)
	}
	if rows == 0 {
		return Service{}, httpapi.ErrNotFound
	}
	return s.GetByID(ctx, serviceID)
}

func (s *PostgresStore) Delete(ctx context.Context, serviceID int) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM services WHERE id = $1`, serviceID)
	if err != nil {
		return fmt.Errorf("delete service %d: %w", serviceID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted service %d: %w", serviceID, err)
	}
	if rows == 0 {
		return httpapi.ErrNotFound
	}
	return nil
}

func (s *PostgresStore) List(ctx context.Context, filter Filter) ([]Service, error) {
	query := strings.Builder{}
	query.WriteString(`
		SELECT id, provider_id, titre, description, categorie, duree_minutes, credits, ville, actif, created_at
		FROM services
		WHERE 1 = 1
	`)
	args := make([]any, 0, 3)

	if filter.Categorie != "" {
		args = append(args, filter.Categorie)
		fmt.Fprintf(&query, " AND categorie = $%d", len(args))
	}
	if filter.Ville != "" {
		args = append(args, filter.Ville)
		fmt.Fprintf(&query, " AND LOWER(ville) = LOWER($%d)", len(args))
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		fmt.Fprintf(&query, " AND (titre ILIKE $%d OR description ILIKE $%d)", len(args), len(args))
	}
	query.WriteString(" ORDER BY created_at DESC, id DESC")

	rows, err := s.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()

	services := make([]Service, 0)
	for rows.Next() {
		var service Service
		var createdAt time.Time
		if err := rows.Scan(
			&service.ID, &service.ProviderID, &service.Titre, &service.Description, &service.Categorie,
			&service.DureeMinutes, &service.Credits, &service.Ville, &service.Actif, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		service.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		services = append(services, service)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate services: %w", err)
	}
	return services, nil
}
