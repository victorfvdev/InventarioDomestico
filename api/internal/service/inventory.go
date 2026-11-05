package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/estoque/api/internal/domain"
	"github.com/estoque/api/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

type InventoryService struct {
	repo *repository.Repository
}

func NewInventoryService(repo *repository.Repository) *InventoryService {
	return &InventoryService{repo: repo}
}

func (s *InventoryService) ListProducts(ctx context.Context, householdID uuid.UUID) ([]domain.Product, error) {
	return s.repo.ListProducts(ctx, householdID)
}

func (s *InventoryService) ListMovements(ctx context.Context, householdID uuid.UUID, limit int) ([]domain.Movement, error) {
	return s.repo.ListMovements(ctx, householdID, limit)
}

func (s *InventoryService) ManualEntry(ctx context.Context, householdID uuid.UUID, userID uuid.UUID, in domain.ManualEntryInput) (domain.Product, domain.Movement, error) {
	var zeroProduct domain.Product
	var zeroMovement domain.Movement

	desc := strings.TrimSpace(in.Description)
	if desc == "" {
		return zeroProduct, zeroMovement, fmt.Errorf("descrição é obrigatória")
	}
	if in.Quantity.IsNegative() {
		return zeroProduct, zeroMovement, fmt.Errorf("quantidade não pode ser negativa")
	}
	if in.Category == "" {
		in.Category = "geral"
	}
	if in.Unit == "" {
		in.Unit = "un"
	}

	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return zeroProduct, zeroMovement, err
	}
	defer tx.Rollback(ctx)

	normalized := domain.NormalizeDescription(desc)
	existing, err := s.repo.FindProductByNormalized(ctx, tx, householdID, normalized)
	if err != nil && err != repository.ErrNotFound {
		return zeroProduct, zeroMovement, err
	}

	var product domain.Product
	var delta decimal.Decimal
	if err == repository.ErrNotFound {
		product = domain.Product{
			ID:                    uuid.New(),
			HouseholdID:           householdID,
			Description:           desc,
			DescriptionNormalized: normalized,
			Category:              in.Category,
			Unit:                  in.Unit,
			Quantity:              in.Quantity,
			MinQuantity:           in.MinQuantity,
			ConfirmationStatus:    domain.StatusConfirmed,
		}
		product, err = s.repo.InsertProduct(ctx, tx, product)
		if err != nil {
			return zeroProduct, zeroMovement, err
		}
		delta = in.Quantity
	} else {
		product = existing
		newQty := existing.Quantity.Add(in.Quantity)
		if err := s.repo.UpdateProductBalance(ctx, tx, product.ID, newQty, domain.StatusConfirmed); err != nil {
			return zeroProduct, zeroMovement, err
		}
		product.Quantity = newQty
		product.ConfirmationStatus = domain.StatusConfirmed
		delta = in.Quantity
	}

	movement := domain.Movement{
		ID:            uuid.New(),
		HouseholdID:   householdID,
		ProductID:     product.ID,
		MovementType:  domain.MovementManualEntry,
		QuantityDelta: delta,
		BalanceAfter:  product.Quantity,
		Origin:        "manual",
		Notes:         in.Notes,
		IsConsumption: false,
		CreatedBy:     &userID,
	}
	movement, err = s.repo.InsertMovement(ctx, tx, movement)
	if err != nil {
		return zeroProduct, zeroMovement, err
	}
	if err := tx.Commit(ctx); err != nil {
		return zeroProduct, zeroMovement, err
	}
	movement.ProductName = product.Description
	return product, movement, nil
}

func (s *InventoryService) InventoryAdjust(ctx context.Context, householdID uuid.UUID, userID uuid.UUID, in domain.InventoryAdjustInput) (domain.Product, domain.Movement, error) {
	var zeroProduct domain.Product
	var zeroMovement domain.Movement
	if in.NewQuantity.IsNegative() {
		return zeroProduct, zeroMovement, fmt.Errorf("quantidade não pode ser negativa")
	}

	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return zeroProduct, zeroMovement, err
	}
	defer tx.Rollback(ctx)

	product, err := getProductForUpdate(ctx, tx, householdID, in.ProductID)
	if err != nil {
		return zeroProduct, zeroMovement, err
	}

	delta := in.NewQuantity.Sub(product.Quantity)
	if err := s.repo.UpdateProductBalance(ctx, tx, product.ID, in.NewQuantity, domain.StatusConfirmed); err != nil {
		return zeroProduct, zeroMovement, err
	}
	product.Quantity = in.NewQuantity
	product.ConfirmationStatus = domain.StatusConfirmed

	notes := in.Notes
	if notes == "" {
		notes = "Ajuste de inventário (não é consumo)"
	}
	movement := domain.Movement{
		ID:            uuid.New(),
		HouseholdID:   householdID,
		ProductID:     product.ID,
		MovementType:  domain.MovementInventoryAdjust,
		QuantityDelta: delta,
		BalanceAfter:  product.Quantity,
		Origin:        "inventory_reconcile",
		Notes:         notes,
		IsConsumption: false, // RN03: ajuste ≠ consumo
		CreatedBy:     &userID,
	}
	movement, err = s.repo.InsertMovement(ctx, tx, movement)
	if err != nil {
		return zeroProduct, zeroMovement, err
	}
	if err := tx.Commit(ctx); err != nil {
		return zeroProduct, zeroMovement, err
	}
	movement.ProductName = product.Description
	return product, movement, nil
}

func (s *InventoryService) ConfirmInitialLoad(ctx context.Context, householdID uuid.UUID, userID uuid.UUID, in domain.ConfirmInitialLoadInput) ([]domain.Product, []domain.Movement, error) {
	if in.InterpretationID == uuid.Nil {
		return nil, nil, fmt.Errorf("interpretationId é obrigatório")
	}

	interp, err := s.repo.GetInterpretation(ctx, householdID, in.InterpretationID)
	if err != nil {
		return nil, nil, err
	}
	if interp.Decision != "pending" {
		return nil, nil, fmt.Errorf("interpretação já foi %s", interp.Decision)
	}

	included := make([]domain.ConfirmInitialLoadItem, 0, len(in.Items))
	for _, item := range in.Items {
		if !item.Included {
			continue
		}
		if strings.TrimSpace(item.Description) == "" {
			return nil, nil, fmt.Errorf("item incluído sem descrição")
		}
		if item.Quantity.IsNegative() {
			return nil, nil, fmt.Errorf("quantidade inválida para %s", item.Description)
		}
		if item.Category == "" {
			item.Category = "geral"
		}
		if item.Unit == "" {
			item.Unit = "un"
		}
		included = append(included, item)
	}
	if len(included) == 0 {
		return nil, nil, fmt.Errorf("selecione ao menos um item para confirmar")
	}

	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	products := make([]domain.Product, 0, len(included))
	movements := make([]domain.Movement, 0, len(included))
	finalSuggestions := make([]domain.VisionSuggestion, 0, len(included))

	for _, item := range included {
		normalized := domain.NormalizeDescription(item.Description)
		existing, err := s.repo.FindProductByNormalized(ctx, tx, householdID, normalized)
		var product domain.Product
		var delta decimal.Decimal

		if err == repository.ErrNotFound {
			product = domain.Product{
				ID:                    uuid.New(),
				HouseholdID:           householdID,
				Description:           strings.TrimSpace(item.Description),
				DescriptionNormalized: normalized,
				Category:              item.Category,
				Unit:                  item.Unit,
				Quantity:              item.Quantity,
				MinQuantity:           item.MinQuantity,
				ConfirmationStatus:    domain.StatusConfirmed,
			}
			product, err = s.repo.InsertProduct(ctx, tx, product)
			if err != nil {
				return nil, nil, err
			}
			delta = item.Quantity
		} else if err != nil {
			return nil, nil, err
		} else {
			product = existing
			newQty := existing.Quantity.Add(item.Quantity)
			if err := s.repo.UpdateProductBalance(ctx, tx, product.ID, newQty, domain.StatusConfirmed); err != nil {
				return nil, nil, err
			}
			product.Quantity = newQty
			product.ConfirmationStatus = domain.StatusConfirmed
			delta = item.Quantity
		}

		notes := item.Notes
		if notes == "" {
			notes = "Carga inicial por foto (confirmada)"
		}
		movement := domain.Movement{
			ID:            uuid.New(),
			HouseholdID:   householdID,
			ProductID:     product.ID,
			MovementType:  domain.MovementInitialLoad,
			QuantityDelta: delta,
			BalanceAfter:  product.Quantity,
			Origin:        fmt.Sprintf("ai_interpretation:%s", in.InterpretationID.String()),
			Notes:         notes,
			IsConsumption: false,
			CreatedBy:     &userID,
		}
		movement, err = s.repo.InsertMovement(ctx, tx, movement)
		if err != nil {
			return nil, nil, err
		}
		movement.ProductName = product.Description
		products = append(products, product)
		movements = append(movements, movement)
		finalSuggestions = append(finalSuggestions, domain.VisionSuggestion{
			TempID:      item.TempID,
			Description: item.Description,
			Category:    item.Category,
			Unit:        item.Unit,
			Quantity:    item.Quantity,
			Included:    true,
			Notes:       item.Notes,
		})
	}

	if err := s.repo.MarkInterpretationConfirmed(ctx, tx, in.InterpretationID, finalSuggestions); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return products, movements, nil
}

func getProductForUpdate(ctx context.Context, tx pgx.Tx, householdID, productID uuid.UUID) (domain.Product, error) {
	var p domain.Product
	err := tx.QueryRow(ctx, `
		SELECT id, household_id, description, description_normalized, category, unit,
		       quantity, min_quantity, confirmation_status, created_at, updated_at
		FROM products
		WHERE household_id=$1 AND id=$2
		FOR UPDATE
	`, householdID, productID).Scan(
		&p.ID, &p.HouseholdID, &p.Description, &p.DescriptionNormalized, &p.Category, &p.Unit,
		&p.Quantity, &p.MinQuantity, &p.ConfirmationStatus, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return p, repository.ErrNotFound
	}
	return p, err
}
