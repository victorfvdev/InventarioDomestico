package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/estoque/api/internal/consumption"
	"github.com/estoque/api/internal/domain"
	"github.com/estoque/api/internal/nfce"
	"github.com/estoque/api/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type ConsumptionService struct {
	repo *repository.Repository
}

func NewConsumptionService(repo *repository.Repository) *ConsumptionService {
	return &ConsumptionService{repo: repo}
}

func (s *ConsumptionService) Interpret(ctx context.Context, householdID uuid.UUID, text string) (domain.InterpretConsumptionResult, error) {
	var zero domain.InterpretConsumptionResult
	text = strings.TrimSpace(text)
	if text == "" {
		return zero, fmt.Errorf("informe o texto do consumo")
	}

	products, err := s.repo.ListProducts(ctx, householdID)
	if err != nil {
		return zero, err
	}

	mentions := consumption.ParseText(text)
	lines := make([]domain.ConsumptionLineSuggestion, 0, len(mentions))

	for _, m := range mentions {
		tempID := uuid.New().String()
		cands := consumption.RankForMention(m.ProductHint, products)
		status, best, score := nfce.DecideMatch(cands)

		line := domain.ConsumptionLineSuggestion{
			TempID:           tempID,
			MentionedText:    m.RawPhrase,
			ProductHint:      m.ProductHint,
			Quantity:         m.Quantity,
			Unit:             m.Unit,
			QuantityInferred: m.QuantityInferred,
			Approx:           m.Approx,
			MatchStatus:      status,
			Candidates:       cands,
			Included:         status == domain.MatchExact || status == domain.MatchSuggested,
			Notes:            "",
		}
		if m.Approx {
			line.Notes = "Quantidade aproximada mencionada no texto"
		}
		if m.QuantityInferred {
			if line.Notes != "" {
				line.Notes += "; "
			}
			line.Notes += "Quantidade sugerida (não informada explicitamente)"
		}

		if best != nil && (status == domain.MatchExact || status == domain.MatchSuggested || status == domain.MatchAmbiguous) {
			id := best.ProductID
			line.MatchedProductID = &id
			line.MatchedProductName = best.Description
			sc := score
			line.MatchConfidence = &sc
			if p, ok := findProduct(products, id); ok {
				line.AvailableQuantity = p.Quantity
				line.InsufficientStock = p.Quantity.LessThan(m.Quantity)
				if line.Unit == "un" && p.Unit != "" {
					line.Unit = p.Unit
				}
			}
		}

		if status == domain.MatchNew || best == nil {
			line.Unmatched = true
			line.Included = false
			line.MatchStatus = domain.MatchNew
			if line.Notes != "" {
				line.Notes += "; "
			}
			line.Notes += "Produto mencionado sem correspondência no estoque — não inventamos item"
		}
		if status == domain.MatchAmbiguous {
			line.Included = false
			if line.Notes != "" {
				line.Notes += "; "
			}
			line.Notes += "Ambiguidade: escolha o produto antes de confirmar"
		}

		lines = append(lines, line)
	}

	interp := domain.ConsumptionInterpretation{
		ID:           uuid.New(),
		HouseholdID:  householdID,
		SourceType:   "consumption_text",
		Provider:     "local-ptbr-parser",
		OriginalText: text,
		Lines:        lines,
		Decision:     "pending",
	}

	raw, _ := json.Marshal(map[string]any{
		"originalText": text,
		"provider":     interp.Provider,
		"mocked":       true,
	})
	stored, err := s.repo.InsertConsumptionInterpretation(ctx, interp, raw)
	if err != nil {
		return zero, err
	}

	return domain.InterpretConsumptionResult{
		Interpretation: stored,
		Provider:       stored.Provider,
		Mocked:         true,
	}, nil
}

func (s *ConsumptionService) Confirm(ctx context.Context, householdID, userID uuid.UUID, in domain.ConfirmConsumptionInput) ([]domain.Movement, error) {
	if in.InterpretationID == uuid.Nil {
		return nil, fmt.Errorf("interpretationId é obrigatório")
	}
	interp, err := s.repo.GetConsumptionInterpretation(ctx, householdID, in.InterpretationID)
	if err != nil {
		return nil, err
	}
	if interp.Decision != "pending" {
		return nil, fmt.Errorf("interpretação já foi %s", interp.Decision)
	}

	included := make([]domain.ConfirmConsumptionItem, 0, len(in.Items))
	for _, item := range in.Items {
		if !item.Included {
			continue
		}
		if item.ProductID == uuid.Nil {
			return nil, fmt.Errorf("produto obrigatório para item incluído")
		}
		if item.Quantity.LessThanOrEqual(decimal.Zero) {
			return nil, fmt.Errorf("quantidade deve ser positiva")
		}
		included = append(included, item)
	}
	if len(included) == 0 {
		return nil, fmt.Errorf("selecione ao menos um item para baixar")
	}

	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	movements := make([]domain.Movement, 0, len(included))
	finalLines := make([]domain.ConsumptionLineSuggestion, 0, len(included))

	for _, item := range included {
		product, err := getProductForUpdate(ctx, tx, householdID, item.ProductID)
		if err != nil {
			return nil, err
		}
		if product.Quantity.LessThan(item.Quantity) {
			return nil, fmt.Errorf("estoque insuficiente para %s (disponível %s, pedido %s)",
				product.Description, product.Quantity.String(), item.Quantity.String())
		}
		newQty := product.Quantity.Sub(item.Quantity)
		if err := s.repo.UpdateProductBalance(ctx, tx, product.ID, newQty, domain.StatusConfirmed); err != nil {
			return nil, err
		}
		notes := item.Notes
		if notes == "" {
			notes = fmt.Sprintf("Consumo confirmado a partir de texto: %q", interp.OriginalText)
		}
		movement := domain.Movement{
			ID:            uuid.New(),
			HouseholdID:   householdID,
			ProductID:     product.ID,
			MovementType:  domain.MovementConsumption,
			QuantityDelta: item.Quantity.Neg(),
			BalanceAfter:  newQty,
			Origin:        fmt.Sprintf("consumption_text:%s", in.InterpretationID.String()),
			Notes:         notes,
			IsConsumption: true, // RN03/RN04: só consumo real após confirmação
			CreatedBy:     &userID,
		}
		movement, err = s.repo.InsertMovement(ctx, tx, movement)
		if err != nil {
			return nil, err
		}
		movement.ProductName = product.Description
		movements = append(movements, movement)
		finalLines = append(finalLines, domain.ConsumptionLineSuggestion{
			TempID:             item.TempID,
			Quantity:           item.Quantity,
			Unit:               item.Unit,
			MatchedProductID:   &product.ID,
			MatchedProductName: product.Description,
			Included:           true,
			Notes:              notes,
		})
	}

	if err := s.repo.MarkConsumptionInterpretationConfirmed(ctx, tx, in.InterpretationID, finalLines); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return movements, nil
}

func (s *ConsumptionService) Quick(ctx context.Context, householdID, userID uuid.UUID, in domain.QuickConsumeInput) (domain.Movement, error) {
	var zero domain.Movement
	if !in.Confirm {
		return zero, fmt.Errorf("confirmação humana obrigatória (confirm=true)")
	}
	if in.ProductID == uuid.Nil {
		return zero, fmt.Errorf("productId é obrigatório")
	}
	if in.Quantity.LessThanOrEqual(decimal.Zero) {
		return zero, fmt.Errorf("quantidade deve ser positiva")
	}

	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return zero, err
	}
	defer tx.Rollback(ctx)

	product, err := getProductForUpdate(ctx, tx, householdID, in.ProductID)
	if err != nil {
		return zero, err
	}
	if product.Quantity.LessThan(in.Quantity) {
		return zero, fmt.Errorf("estoque insuficiente para %s (disponível %s)", product.Description, product.Quantity.String())
	}
	newQty := product.Quantity.Sub(in.Quantity)
	if err := s.repo.UpdateProductBalance(ctx, tx, product.ID, newQty, domain.StatusConfirmed); err != nil {
		return zero, err
	}
	notes := in.Notes
	if notes == "" {
		notes = "Consumo rápido (atalho confirmado)"
	}
	unit := in.Unit
	if unit == "" {
		unit = product.Unit
	}
	_ = unit
	movement := domain.Movement{
		ID:            uuid.New(),
		HouseholdID:   householdID,
		ProductID:     product.ID,
		MovementType:  domain.MovementConsumption,
		QuantityDelta: in.Quantity.Neg(),
		BalanceAfter:  newQty,
		Origin:        "consumption_shortcut",
		Notes:         notes,
		IsConsumption: true,
		CreatedBy:     &userID,
	}
	movement, err = s.repo.InsertMovement(ctx, tx, movement)
	if err != nil {
		return zero, err
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, err
	}
	movement.ProductName = product.Description
	return movement, nil
}

// Undo reverses a recent consumption with an inventory_adjust (not counted as consumption).
func (s *ConsumptionService) Undo(ctx context.Context, householdID, userID uuid.UUID, movementID uuid.UUID) (domain.Movement, error) {
	var zero domain.Movement
	if movementID == uuid.Nil {
		return zero, fmt.Errorf("movementId é obrigatório")
	}
	orig, err := s.repo.GetMovement(ctx, householdID, movementID)
	if err != nil {
		return zero, err
	}
	if orig.MovementType != domain.MovementConsumption || !orig.IsConsumption {
		return zero, fmt.Errorf("só é possível desfazer movimentações de consumo")
	}
	if strings.Contains(orig.Notes, "[desfeito]") {
		return zero, fmt.Errorf("este consumo já foi desfeito")
	}
	// quantity_delta is negative for consumption; restore the absolute amount
	restore := orig.QuantityDelta.Abs()

	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return zero, err
	}
	defer tx.Rollback(ctx)

	product, err := getProductForUpdate(ctx, tx, householdID, orig.ProductID)
	if err != nil {
		return zero, err
	}
	newQty := product.Quantity.Add(restore)
	if err := s.repo.UpdateProductBalance(ctx, tx, product.ID, newQty, domain.StatusConfirmed); err != nil {
		return zero, err
	}
	movement := domain.Movement{
		ID:            uuid.New(),
		HouseholdID:   householdID,
		ProductID:     product.ID,
		MovementType:  domain.MovementInventoryAdjust,
		QuantityDelta: restore,
		BalanceAfter:  newQty,
		Origin:        fmt.Sprintf("undo_consumption:%s", orig.ID.String()),
		Notes:         fmt.Sprintf("Desfazer consumo equivocado (ajuste ≠ consumo). Original: %s", orig.ID.String()),
		IsConsumption: false, // RN03: ajuste não é consumo
		CreatedBy:     &userID,
	}
	movement, err = s.repo.InsertMovement(ctx, tx, movement)
	if err != nil {
		return zero, err
	}
	if err := s.repo.MarkMovementNotes(ctx, tx, orig.ID, orig.Notes+" [desfeito]"); err != nil {
		return zero, err
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, err
	}
	movement.ProductName = product.Description
	return movement, nil
}

func (s *ConsumptionService) Shortcuts(ctx context.Context, householdID uuid.UUID) ([]domain.ConsumptionShortcut, error) {
	products, err := s.repo.ListProducts(ctx, householdID)
	if err != nil {
		return nil, err
	}
	byID := map[uuid.UUID]domain.Product{}
	for _, p := range products {
		byID[p.ID] = p
	}

	freq, err := s.repo.FrequentConsumptions(ctx, householdID, 8)
	if err != nil {
		return nil, err
	}

	out := make([]domain.ConsumptionShortcut, 0, 8)
	seen := map[uuid.UUID]bool{}

	for _, f := range freq {
		p, ok := byID[f.ProductID]
		if !ok || p.Quantity.LessThanOrEqual(decimal.Zero) {
			continue
		}
		qty := f.TypicalQty
		if qty.LessThanOrEqual(decimal.Zero) {
			qty = decimal.NewFromInt(1)
		}
		if qty.GreaterThan(p.Quantity) {
			qty = p.Quantity
		}
		out = append(out, domain.ConsumptionShortcut{
			ID:          uuid.New().String(),
			Label:       fmt.Sprintf("Consumir %s %s", qty.String(), p.Description),
			ProductID:   p.ID,
			ProductName: p.Description,
			Quantity:    qty,
			Unit:        p.Unit,
			Source:      "history",
			Available:   p.Quantity,
		})
		seen[p.ID] = true
		if len(out) >= 6 {
			break
		}
	}

	// Seeds when history is thin — only for products that exist; never invent.
	seeds := []struct {
		hints []string
		qty   decimal.Decimal
		label string
	}{
		{[]string{"ovo", "ovos"}, decimal.NewFromInt(2), "Consumir 2 ovos"},
		{[]string{"banana", "bananas"}, decimal.NewFromInt(1), "Consumir 1 banana"},
		{[]string{"leite"}, decimal.NewFromInt(1), "Consumir 1 unidade de leite"},
		{[]string{"creme de leite"}, decimal.NewFromInt(1), "Consumir 1 caixa de creme de leite"},
		{[]string{"pao", "pão", "pao de forma"}, decimal.NewFromInt(2), "Consumir 2 pães"},
		{[]string{"cafe", "café"}, decimal.NewFromInt(1), "Consumir 1 café"},
	}
	for _, seed := range seeds {
		if len(out) >= 6 {
			break
		}
		var matched *domain.Product
		var bestScore float64
		for _, h := range seed.hints {
			cands := consumption.RankForMention(h, products)
			for _, c := range cands {
				if seen[c.ProductID] || c.Score < 0.55 || c.Score <= bestScore {
					continue
				}
				p, ok := byID[c.ProductID]
				if !ok || p.Quantity.LessThanOrEqual(decimal.Zero) {
					continue
				}
				cp := p
				matched = &cp
				bestScore = c.Score
			}
		}
		if matched == nil {
			continue
		}
		qty := seed.qty
		if qty.GreaterThan(matched.Quantity) {
			qty = matched.Quantity
		}
		out = append(out, domain.ConsumptionShortcut{
			ID:          uuid.New().String(),
			Label:       seed.label,
			ProductID:   matched.ID,
			ProductName: matched.Description,
			Quantity:    qty,
			Unit:        matched.Unit,
			Source:      "seed",
			Available:   matched.Quantity,
		})
		seen[matched.ID] = true
	}
	if out == nil {
		out = []domain.ConsumptionShortcut{}
	}
	return out, nil
}

func findProduct(products []domain.Product, id uuid.UUID) (domain.Product, bool) {
	for _, p := range products {
		if p.ID == id {
			return p, true
		}
	}
	return domain.Product{}, false
}
