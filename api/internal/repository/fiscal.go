package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/estoque/api/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) FindFiscalByAccessKey(ctx context.Context, householdID uuid.UUID, accessKey string) (domain.FiscalDocument, error) {
	var doc domain.FiscalDocument
	err := r.pool.QueryRow(ctx, `
		SELECT id, household_id, access_key, uf, document_type, issuer_name, issuer_cnpj,
		       issued_at, total_amount, status, source, COALESCE(fetch_notes,''), created_at, confirmed_at
		FROM fiscal_documents
		WHERE household_id=$1 AND access_key=$2
	`, householdID, accessKey).Scan(
		&doc.ID, &doc.HouseholdID, &doc.AccessKey, &doc.UF, &doc.DocumentType, &doc.IssuerName, &doc.IssuerCNPJ,
		&doc.IssuedAt, &doc.TotalAmount, &doc.Status, &doc.Source, &doc.FetchNotes, &doc.CreatedAt, &doc.ConfirmedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return doc, ErrNotFound
	}
	if err != nil {
		return doc, err
	}
	if doc.Status == domain.FiscalConfirmed {
		doc.AlreadyPosted = true
	}
	return doc, nil
}

func (r *Repository) InsertFiscalDocument(ctx context.Context, tx pgx.Tx, doc domain.FiscalDocument, raw json.RawMessage) (domain.FiscalDocument, error) {
	if raw == nil {
		raw = json.RawMessage(`{}`)
	}
	err := tx.QueryRow(ctx, `
		INSERT INTO fiscal_documents(
			id, household_id, access_key, uf, document_type, issuer_name, issuer_cnpj,
			issued_at, total_amount, status, source, raw_payload, fetch_notes
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id, household_id, access_key, uf, document_type, issuer_name, issuer_cnpj,
		          issued_at, total_amount, status, source, COALESCE(fetch_notes,''), created_at, confirmed_at
	`, doc.ID, doc.HouseholdID, doc.AccessKey, doc.UF, doc.DocumentType, doc.IssuerName, doc.IssuerCNPJ,
		doc.IssuedAt, doc.TotalAmount, doc.Status, doc.Source, raw, nullIfEmpty(doc.FetchNotes),
	).Scan(
		&doc.ID, &doc.HouseholdID, &doc.AccessKey, &doc.UF, &doc.DocumentType, &doc.IssuerName, &doc.IssuerCNPJ,
		&doc.IssuedAt, &doc.TotalAmount, &doc.Status, &doc.Source, &doc.FetchNotes, &doc.CreatedAt, &doc.ConfirmedAt,
	)
	return doc, err
}

func (r *Repository) InsertFiscalItem(ctx context.Context, tx pgx.Tx, item domain.FiscalItem) (domain.FiscalItem, error) {
	err := tx.QueryRow(ctx, `
		INSERT INTO fiscal_document_items(
			id, fiscal_document_id, line_number, description, description_normalized,
			quantity, unit, unit_price, total_price, ean, category,
			match_status, match_confidence, matched_product_id, create_new, included
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING id, fiscal_document_id, line_number, description, description_normalized,
		          quantity, unit, unit_price, total_price, COALESCE(ean,''), category,
		          match_status, match_confidence, matched_product_id, create_new, included
	`, item.ID, item.FiscalDocumentID, item.LineNumber, item.Description, item.DescriptionNormalized,
		item.Quantity, item.Unit, item.UnitPrice, item.TotalPrice, nullIfEmpty(item.EAN), item.Category,
		item.MatchStatus, item.MatchConfidence, item.MatchedProductID, item.CreateNew, item.Included,
	).Scan(
		&item.ID, &item.FiscalDocumentID, &item.LineNumber, &item.Description, &item.DescriptionNormalized,
		&item.Quantity, &item.Unit, &item.UnitPrice, &item.TotalPrice, &item.EAN, &item.Category,
		&item.MatchStatus, &item.MatchConfidence, &item.MatchedProductID, &item.CreateNew, &item.Included,
	)
	return item, err
}

func (r *Repository) GetFiscalDocument(ctx context.Context, householdID, id uuid.UUID) (domain.FiscalDocument, error) {
	var doc domain.FiscalDocument
	err := r.pool.QueryRow(ctx, `
		SELECT id, household_id, access_key, uf, document_type, issuer_name, issuer_cnpj,
		       issued_at, total_amount, status, source, COALESCE(fetch_notes,''), created_at, confirmed_at
		FROM fiscal_documents
		WHERE household_id=$1 AND id=$2
	`, householdID, id).Scan(
		&doc.ID, &doc.HouseholdID, &doc.AccessKey, &doc.UF, &doc.DocumentType, &doc.IssuerName, &doc.IssuerCNPJ,
		&doc.IssuedAt, &doc.TotalAmount, &doc.Status, &doc.Source, &doc.FetchNotes, &doc.CreatedAt, &doc.ConfirmedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return doc, ErrNotFound
	}
	if err != nil {
		return doc, err
	}
	doc.AlreadyPosted = doc.Status == domain.FiscalConfirmed
	items, err := r.ListFiscalItems(ctx, doc.ID)
	if err != nil {
		return doc, err
	}
	doc.Items = items
	return doc, nil
}

func (r *Repository) ListFiscalItems(ctx context.Context, documentID uuid.UUID) ([]domain.FiscalItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT i.id, i.fiscal_document_id, i.line_number, i.description, i.description_normalized,
		       i.quantity, i.unit, i.unit_price, i.total_price, COALESCE(i.ean,''), i.category,
		       i.match_status, i.match_confidence, i.matched_product_id, i.create_new, i.included,
		       COALESCE(p.description, '')
		FROM fiscal_document_items i
		LEFT JOIN products p ON p.id = i.matched_product_id
		WHERE i.fiscal_document_id=$1
		ORDER BY i.line_number ASC
	`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.FiscalItem
	for rows.Next() {
		var item domain.FiscalItem
		if err := rows.Scan(
			&item.ID, &item.FiscalDocumentID, &item.LineNumber, &item.Description, &item.DescriptionNormalized,
			&item.Quantity, &item.Unit, &item.UnitPrice, &item.TotalPrice, &item.EAN, &item.Category,
			&item.MatchStatus, &item.MatchConfidence, &item.MatchedProductID, &item.CreateNew, &item.Included,
			&item.MatchedProductName,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if out == nil {
		out = []domain.FiscalItem{}
	}
	return out, rows.Err()
}

func (r *Repository) GetFiscalItem(ctx context.Context, tx pgx.Tx, documentID, itemID uuid.UUID) (domain.FiscalItem, error) {
	var item domain.FiscalItem
	err := tx.QueryRow(ctx, `
		SELECT id, fiscal_document_id, line_number, description, description_normalized,
		       quantity, unit, unit_price, total_price, COALESCE(ean,''), category,
		       match_status, match_confidence, matched_product_id, create_new, included
		FROM fiscal_document_items
		WHERE fiscal_document_id=$1 AND id=$2
		FOR UPDATE
	`, documentID, itemID).Scan(
		&item.ID, &item.FiscalDocumentID, &item.LineNumber, &item.Description, &item.DescriptionNormalized,
		&item.Quantity, &item.Unit, &item.UnitPrice, &item.TotalPrice, &item.EAN, &item.Category,
		&item.MatchStatus, &item.MatchConfidence, &item.MatchedProductID, &item.CreateNew, &item.Included,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}

func (r *Repository) UpdateFiscalItemConfirmation(ctx context.Context, tx pgx.Tx, itemID, productID, movementID uuid.UUID, included bool) error {
	_, err := tx.Exec(ctx, `
		UPDATE fiscal_document_items
		SET included=$2, confirmed_product_id=$3, movement_id=$4, create_new=false, match_status='manual'
		WHERE id=$1
	`, itemID, included, productID, movementID)
	return err
}

func (r *Repository) MarkFiscalConfirmed(ctx context.Context, tx pgx.Tx, documentID uuid.UUID) error {
	ct, err := tx.Exec(ctx, `
		UPDATE fiscal_documents
		SET status='confirmed', confirmed_at=$2
		WHERE id=$1 AND status IN ('parsed','fetched','draft')
	`, documentID, time.Now().UTC())
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) LockFiscalForConfirm(ctx context.Context, tx pgx.Tx, householdID, documentID uuid.UUID) (domain.FiscalDocument, error) {
	var doc domain.FiscalDocument
	err := tx.QueryRow(ctx, `
		SELECT id, household_id, access_key, uf, document_type, issuer_name, issuer_cnpj,
		       issued_at, total_amount, status, source, COALESCE(fetch_notes,''), created_at, confirmed_at
		FROM fiscal_documents
		WHERE household_id=$1 AND id=$2
		FOR UPDATE
	`, householdID, documentID).Scan(
		&doc.ID, &doc.HouseholdID, &doc.AccessKey, &doc.UF, &doc.DocumentType, &doc.IssuerName, &doc.IssuerCNPJ,
		&doc.IssuedAt, &doc.TotalAmount, &doc.Status, &doc.Source, &doc.FetchNotes, &doc.CreatedAt, &doc.ConfirmedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return doc, ErrNotFound
	}
	return doc, err
}

func (r *Repository) DeleteFiscalDraft(ctx context.Context, tx pgx.Tx, documentID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM fiscal_documents WHERE id=$1 AND status <> 'confirmed'`, documentID)
	return err
}
