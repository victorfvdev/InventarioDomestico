package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/estoque/api/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

var ErrNotFound = errors.New("not found")

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

func (r *Repository) EnsureBootstrap(ctx context.Context, householdName, userName string) (domain.SessionInfo, error) {
	var session domain.SessionInfo
	err := r.pool.QueryRow(ctx, `SELECT id, name, created_at FROM households ORDER BY created_at ASC LIMIT 1`).
		Scan(&session.Household.ID, &session.Household.Name, &session.Household.CreatedAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return session, err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		err = r.pool.QueryRow(ctx, `
			INSERT INTO households(name) VALUES ($1)
			RETURNING id, name, created_at
		`, householdName).Scan(&session.Household.ID, &session.Household.Name, &session.Household.CreatedAt)
		if err != nil {
			return session, err
		}
	}

	err = r.pool.QueryRow(ctx, `
		SELECT id, household_id, display_name, created_at
		FROM users WHERE household_id=$1 ORDER BY created_at ASC LIMIT 1
	`, session.Household.ID).Scan(&session.User.ID, &session.User.HouseholdID, &session.User.DisplayName, &session.User.CreatedAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return session, err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		err = r.pool.QueryRow(ctx, `
			INSERT INTO users(household_id, display_name) VALUES ($1, $2)
			RETURNING id, household_id, display_name, created_at
		`, session.Household.ID, userName).Scan(&session.User.ID, &session.User.HouseholdID, &session.User.DisplayName, &session.User.CreatedAt)
		if err != nil {
			return session, err
		}
	}
	return session, nil
}

func (r *Repository) ListProducts(ctx context.Context, householdID uuid.UUID) ([]domain.Product, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, household_id, description, description_normalized, category, unit,
		       quantity, min_quantity, confirmation_status, created_at, updated_at
		FROM products
		WHERE household_id=$1
		ORDER BY description ASC
	`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if out == nil {
		out = []domain.Product{}
	}
	return out, rows.Err()
}

func (r *Repository) GetProduct(ctx context.Context, householdID, productID uuid.UUID) (domain.Product, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, household_id, description, description_normalized, category, unit,
		       quantity, min_quantity, confirmation_status, created_at, updated_at
		FROM products WHERE household_id=$1 AND id=$2
	`, householdID, productID)
	p, err := scanProduct(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

func (r *Repository) FindProductByNormalized(ctx context.Context, tx pgx.Tx, householdID uuid.UUID, normalized string) (domain.Product, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, household_id, description, description_normalized, category, unit,
		       quantity, min_quantity, confirmation_status, created_at, updated_at
		FROM products WHERE household_id=$1 AND description_normalized=$2
		FOR UPDATE
	`, householdID, normalized)
	p, err := scanProduct(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

func (r *Repository) InsertProduct(ctx context.Context, tx pgx.Tx, p domain.Product) (domain.Product, error) {
	err := tx.QueryRow(ctx, `
		INSERT INTO products(
			id, household_id, description, description_normalized, category, unit,
			quantity, min_quantity, confirmation_status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, household_id, description, description_normalized, category, unit,
		          quantity, min_quantity, confirmation_status, created_at, updated_at
	`, p.ID, p.HouseholdID, p.Description, p.DescriptionNormalized, p.Category, p.Unit,
		p.Quantity, p.MinQuantity, p.ConfirmationStatus).Scan(
		&p.ID, &p.HouseholdID, &p.Description, &p.DescriptionNormalized, &p.Category, &p.Unit,
		&p.Quantity, &p.MinQuantity, &p.ConfirmationStatus, &p.CreatedAt, &p.UpdatedAt,
	)
	return p, err
}

func (r *Repository) UpdateProductBalance(ctx context.Context, tx pgx.Tx, productID uuid.UUID, qty decimal.Decimal, status domain.ConfirmationStatus) error {
	ct, err := tx.Exec(ctx, `
		UPDATE products
		SET quantity=$2, confirmation_status=$3, updated_at=now()
		WHERE id=$1
	`, productID, qty, status)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) InsertMovement(ctx context.Context, tx pgx.Tx, m domain.Movement) (domain.Movement, error) {
	err := tx.QueryRow(ctx, `
		INSERT INTO movements(
			id, household_id, product_id, movement_type, quantity_delta, balance_after,
			origin, notes, is_consumption, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id, household_id, product_id, movement_type, quantity_delta, balance_after,
		          origin, COALESCE(notes,''), is_consumption, created_by, created_at
	`, m.ID, m.HouseholdID, m.ProductID, m.MovementType, m.QuantityDelta, m.BalanceAfter,
		m.Origin, nullIfEmpty(m.Notes), m.IsConsumption, m.CreatedBy).Scan(
		&m.ID, &m.HouseholdID, &m.ProductID, &m.MovementType, &m.QuantityDelta, &m.BalanceAfter,
		&m.Origin, &m.Notes, &m.IsConsumption, &m.CreatedBy, &m.CreatedAt,
	)
	return m, err
}

func (r *Repository) ListMovements(ctx context.Context, householdID uuid.UUID, limit int) ([]domain.Movement, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.household_id, m.product_id, m.movement_type, m.quantity_delta, m.balance_after,
		       m.origin, COALESCE(m.notes,''), m.is_consumption, m.created_by, m.created_at,
		       p.description
		FROM movements m
		JOIN products p ON p.id = m.product_id
		WHERE m.household_id=$1
		ORDER BY m.created_at DESC
		LIMIT $2
	`, householdID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Movement
	for rows.Next() {
		var m domain.Movement
		if err := rows.Scan(
			&m.ID, &m.HouseholdID, &m.ProductID, &m.MovementType, &m.QuantityDelta, &m.BalanceAfter,
			&m.Origin, &m.Notes, &m.IsConsumption, &m.CreatedBy, &m.CreatedAt, &m.ProductName,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if out == nil {
		out = []domain.Movement{}
	}
	return out, rows.Err()
}

func (r *Repository) InsertInterpretation(ctx context.Context, interp domain.AIInterpretation, raw json.RawMessage) (domain.AIInterpretation, error) {
	suggestions, err := json.Marshal(interp.Suggestions)
	if err != nil {
		return interp, err
	}
	if raw == nil {
		raw = json.RawMessage(`{}`)
	}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO ai_interpretations(
			id, household_id, source_type, image_filename, provider, raw_response, suggestions,
			decision, overall_confidence
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, household_id, source_type, COALESCE(image_filename,''), provider, decision, overall_confidence, created_at, confirmed_at
	`, interp.ID, interp.HouseholdID, interp.SourceType, nullIfEmpty(interp.ImageFilename), interp.Provider,
		raw, suggestions, interp.Decision, interp.OverallConfidence).Scan(
		&interp.ID, &interp.HouseholdID, &interp.SourceType, &interp.ImageFilename, &interp.Provider,
		&interp.Decision, &interp.OverallConfidence, &interp.CreatedAt, &interp.ConfirmedAt,
	)
	return interp, err
}

func (r *Repository) GetInterpretation(ctx context.Context, householdID, id uuid.UUID) (domain.AIInterpretation, error) {
	var interp domain.AIInterpretation
	var suggestions []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, household_id, source_type, COALESCE(image_filename,''), provider, suggestions,
		       decision, overall_confidence, created_at, confirmed_at
		FROM ai_interpretations
		WHERE household_id=$1 AND id=$2
	`, householdID, id).Scan(
		&interp.ID, &interp.HouseholdID, &interp.SourceType, &interp.ImageFilename, &interp.Provider,
		&suggestions, &interp.Decision, &interp.OverallConfidence, &interp.CreatedAt, &interp.ConfirmedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return interp, ErrNotFound
	}
	if err != nil {
		return interp, err
	}
	if err := json.Unmarshal(suggestions, &interp.Suggestions); err != nil {
		return interp, fmt.Errorf("decode suggestions: %w", err)
	}
	return interp, nil
}

func (r *Repository) MarkInterpretationConfirmed(ctx context.Context, tx pgx.Tx, id uuid.UUID, suggestions []domain.VisionSuggestion) error {
	body, err := json.Marshal(suggestions)
	if err != nil {
		return err
	}
	ct, err := tx.Exec(ctx, `
		UPDATE ai_interpretations
		SET decision='confirmed', confirmed_at=$2, suggestions=$3
		WHERE id=$1 AND decision='pending'
	`, id, time.Now().UTC(), body)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanProduct(row scannable) (domain.Product, error) {
	var p domain.Product
	err := row.Scan(
		&p.ID, &p.HouseholdID, &p.Description, &p.DescriptionNormalized, &p.Category, &p.Unit,
		&p.Quantity, &p.MinQuantity, &p.ConfirmationStatus, &p.CreatedAt, &p.UpdatedAt,
	)
	return p, err
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
