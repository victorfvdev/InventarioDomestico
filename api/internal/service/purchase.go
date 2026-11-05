package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/estoque/api/internal/domain"
	"github.com/estoque/api/internal/nfce"
	"github.com/estoque/api/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PurchaseService struct {
	repo    *repository.Repository
	adapter *nfce.Adapter
}

func NewPurchaseService(repo *repository.Repository, adapter *nfce.Adapter) *PurchaseService {
	return &PurchaseService{repo: repo, adapter: adapter}
}

type LookupResponse struct {
	Document    domain.FiscalDocument `json:"document"`
	LiveTried   bool                  `json:"liveTried"`
	LiveOK      bool                  `json:"liveOk"`
	UsedFixture bool                  `json:"usedFixture"`
	Notes       string                `json:"notes"`
	SefazLimits []string              `json:"sefazLimits"`
}

func sefazLimits() []string {
	return []string{
		"Consulta pública MT frequentemente exige CAPTCHA/sessão — automação costuma falhar.",
		"Não há API JSON estável; o parser depende do HTML da consulta.",
		"Indisponibilidade e mudanças de URL/layout da SEFAZ-MT são esperadas.",
		"Use a fixture offline ou entrada manual quando a consulta real bloquear.",
	}
}

func (s *PurchaseService) Lookup(ctx context.Context, householdID uuid.UUID, in domain.NFCeLookupInput) (LookupResponse, error) {
	raw := strings.TrimSpace(in.AccessKey)
	if raw == "" {
		raw = strings.TrimSpace(in.QRUrl)
	}
	if raw == "" && in.UseFixture {
		raw = nfce.FixtureAccessKey
	}
	if raw == "" {
		return LookupResponse{}, fmt.Errorf("informe accessKey, qrUrl ou useFixture=true")
	}

	key, err := nfce.ExtractAccessKey(raw)
	if err != nil {
		return LookupResponse{}, err
	}

	// Idempotência: se já confirmada, devolve o documento sem reprocessar.
	var existing domain.FiscalDocument
	found, err := s.repo.FindFiscalByAccessKey(ctx, householdID, key)
	if err != nil && err != repository.ErrNotFound {
		return LookupResponse{}, err
	}
	if err == nil {
		existing = found
		if existing.Status == domain.FiscalConfirmed {
			full, err := s.repo.GetFiscalDocument(ctx, householdID, existing.ID)
			if err != nil {
				return LookupResponse{}, err
			}
			full.AlreadyPosted = true
			return LookupResponse{
				Document:    full,
				Notes:       "Esta chave já gerou entrada de estoque. Nenhuma movimentação adicional foi criada.",
				SefazLimits: sefazLimits(),
			}, nil
		}
	}

	fetch, err := s.adapter.FetchByAccessKey(ctx, key, in.UseFixture)
	if err != nil {
		return LookupResponse{}, err
	}

	doc, err := s.persistParsed(ctx, householdID, fetch, existing)
	if err != nil {
		return LookupResponse{}, err
	}
	return LookupResponse{
		Document:    doc,
		LiveTried:   fetch.LiveTried,
		LiveOK:      fetch.LiveOK,
		UsedFixture: fetch.UsedFixture,
		Notes:       fetch.Notes,
		SefazLimits: sefazLimits(),
	}, nil
}

func (s *PurchaseService) persistParsed(ctx context.Context, householdID uuid.UUID, fetch nfce.FetchResult, existing domain.FiscalDocument) (domain.FiscalDocument, error) {
	products, err := s.repo.ListProducts(ctx, householdID)
	if err != nil {
		return domain.FiscalDocument{}, err
	}

	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return domain.FiscalDocument{}, err
	}
	defer tx.Rollback(ctx)

	// Replace non-confirmed draft for same key
	if existing.ID != uuid.Nil && existing.Status != domain.FiscalConfirmed {
		if err := s.repo.DeleteFiscalDraft(ctx, tx, existing.ID); err != nil {
			return domain.FiscalDocument{}, err
		}
	}

	parsed := fetch.Parsed
	doc := domain.FiscalDocument{
		ID:           uuid.New(),
		HouseholdID:  householdID,
		AccessKey:    parsed.AccessKey,
		UF:           "MT",
		DocumentType: "nfce",
		IssuerName:   parsed.IssuerName,
		IssuerCNPJ:   parsed.IssuerCNPJ,
		IssuedAt:     parsed.IssuedAt,
		TotalAmount:  parsed.Total,
		Status:       domain.FiscalParsed,
		Source:       fetch.Source,
		FetchNotes:   fetch.Notes,
	}
	raw, _ := json.Marshal(map[string]any{
		"htmlSnippet": truncate(fetch.HTML, 4000),
		"notes":       fetch.Notes,
		"liveTried":   fetch.LiveTried,
		"liveOk":      fetch.LiveOK,
		"liveError":   fetch.LiveError,
	})
	doc, err = s.repo.InsertFiscalDocument(ctx, tx, doc, raw)
	if err != nil {
		return domain.FiscalDocument{}, err
	}

	items := make([]domain.FiscalItem, 0, len(parsed.Items))
	for _, pi := range parsed.Items {
		norm := domain.NormalizeDescription(pi.Description)
		candidates := nfce.RankProducts(norm, products)
		status, best, score := nfce.DecideMatch(candidates)
		item := domain.FiscalItem{
			ID:                    uuid.New(),
			FiscalDocumentID:      doc.ID,
			LineNumber:            pi.LineNumber,
			Description:           pi.Description,
			DescriptionNormalized: norm,
			Quantity:              pi.Quantity,
			Unit:                  normalizeUnit(pi.Unit),
			UnitPrice:             pi.UnitPrice,
			TotalPrice:            pi.TotalPrice,
			EAN:                   pi.EAN,
			Category:              nfce.GuessCategory(pi.Description),
			MatchStatus:           status,
			CreateNew:             status == domain.MatchNew,
			Included:              true,
			AmbiguousCandidates:   candidates,
		}
		if score > 0 {
			sc := score
			item.MatchConfidence = &sc
		}
		if best != nil && status != domain.MatchNew {
			id := best.ProductID
			item.MatchedProductID = &id
			item.MatchedProductName = best.Description
			item.CreateNew = false
		}
		// Ambiguidade exige confirmação humana — não pré-seleciona produto final sem review
		if status == domain.MatchAmbiguous {
			item.CreateNew = false
		}
		item, err = s.repo.InsertFiscalItem(ctx, tx, item)
		if err != nil {
			return domain.FiscalDocument{}, err
		}
		item.AmbiguousCandidates = candidates
		items = append(items, item)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.FiscalDocument{}, err
	}
	doc.Items = items
	return doc, nil
}

func (s *PurchaseService) ManualDraft(ctx context.Context, householdID uuid.UUID, in domain.ManualPurchaseInput) (domain.FiscalDocument, error) {
	if len(in.Items) == 0 {
		return domain.FiscalDocument{}, fmt.Errorf("informe ao menos um item")
	}
	accessKey := ""
	if strings.TrimSpace(in.AccessKey) != "" {
		key, err := nfce.ExtractAccessKey(in.AccessKey)
		if err != nil {
			return domain.FiscalDocument{}, err
		}
		if !nfce.IsMatoGrosso(key) {
			return domain.FiscalDocument{}, fmt.Errorf("chave informada não é de MT (esperado UF 51)")
		}
		existing, err := s.repo.FindFiscalByAccessKey(ctx, householdID, key)
		if err != nil && err != repository.ErrNotFound {
			return domain.FiscalDocument{}, err
		}
		if err == nil && existing.Status == domain.FiscalConfirmed {
			return domain.FiscalDocument{}, fmt.Errorf("esta chave já gerou entrada de estoque — não é possível republicar")
		}
		accessKey = key
	}

	products, err := s.repo.ListProducts(ctx, householdID)
	if err != nil {
		return domain.FiscalDocument{}, err
	}

	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return domain.FiscalDocument{}, err
	}
	defer tx.Rollback(ctx)

	if accessKey != "" {
		if existing, err := s.repo.FindFiscalByAccessKey(ctx, householdID, accessKey); err == nil && existing.Status != domain.FiscalConfirmed {
			_ = s.repo.DeleteFiscalDraft(ctx, tx, existing.ID)
		}
	}

	total := decimal.Zero
	source := strings.TrimSpace(in.Source)
	if source == "" {
		source = "manual"
	}
	doc := domain.FiscalDocument{
		ID:           uuid.New(),
		HouseholdID:  householdID,
		AccessKey:    accessKey,
		UF:           "MT",
		DocumentType: "nfce",
		IssuerName:   strings.TrimSpace(in.IssuerName),
		Status:       domain.FiscalDraft,
		Source:       source,
		FetchNotes:   in.Notes,
	}
	if doc.IssuerName == "" {
		doc.IssuerName = "Entrada manual"
	}

	parsedItems := make([]nfce.ParsedItem, 0, len(in.Items))
	for i, it := range in.Items {
		desc := strings.TrimSpace(it.Description)
		if desc == "" {
			return domain.FiscalDocument{}, fmt.Errorf("item %d sem descrição", i+1)
		}
		if it.Quantity.IsNegative() || it.Quantity.IsZero() {
			return domain.FiscalDocument{}, fmt.Errorf("quantidade inválida no item %d", i+1)
		}
		unit := normalizeUnit(it.Unit)
		lineTotal := it.Quantity.Mul(it.UnitPrice)
		total = total.Add(lineTotal)
		parsedItems = append(parsedItems, nfce.ParsedItem{
			LineNumber:  i + 1,
			Description: desc,
			Quantity:    it.Quantity,
			Unit:        unit,
			UnitPrice:   it.UnitPrice,
			TotalPrice:  lineTotal,
			EAN:         it.EAN,
		})
	}
	doc.TotalAmount = total

	raw, _ := json.Marshal(map[string]any{"manual": true, "notes": in.Notes})
	doc, err = s.repo.InsertFiscalDocument(ctx, tx, doc, raw)
	if err != nil {
		return domain.FiscalDocument{}, err
	}

	items := make([]domain.FiscalItem, 0, len(parsedItems))
	for i, pi := range parsedItems {
		cat := in.Items[i].Category
		if cat == "" {
			cat = nfce.GuessCategory(pi.Description)
		}
		norm := domain.NormalizeDescription(pi.Description)
		candidates := nfce.RankProducts(norm, products)
		status, best, score := nfce.DecideMatch(candidates)
		if status == domain.MatchNew {
			status = domain.MatchManual
		}
		item := domain.FiscalItem{
			ID:                    uuid.New(),
			FiscalDocumentID:      doc.ID,
			LineNumber:            pi.LineNumber,
			Description:           pi.Description,
			DescriptionNormalized: norm,
			Quantity:              pi.Quantity,
			Unit:                  pi.Unit,
			UnitPrice:             pi.UnitPrice,
			TotalPrice:            pi.TotalPrice,
			EAN:                   pi.EAN,
			Category:              cat,
			MatchStatus:           status,
			CreateNew:             best == nil,
			Included:              true,
			AmbiguousCandidates:   candidates,
		}
		if score > 0 {
			sc := score
			item.MatchConfidence = &sc
		}
		if best != nil {
			id := best.ProductID
			item.MatchedProductID = &id
			item.MatchedProductName = best.Description
			item.CreateNew = false
		}
		item, err = s.repo.InsertFiscalItem(ctx, tx, item)
		if err != nil {
			return domain.FiscalDocument{}, err
		}
		item.AmbiguousCandidates = candidates
		items = append(items, item)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.FiscalDocument{}, err
	}
	doc.Items = items
	return doc, nil
}

func (s *PurchaseService) PhotoStub(ctx context.Context, householdID uuid.UUID, filename string) (domain.FiscalDocument, error) {
	// Stub sem OpenAI: devolve itens típicos de cupom MT para revisão humana.
	in := domain.ManualPurchaseInput{
		IssuerName: "Cupom fotografado (stub local)",
		Source:     "photo_stub",
		Notes:      fmt.Sprintf("Foto %s interpretada por stub local — sem OpenAI. Revise os itens antes de confirmar.", filename),
		Items: []domain.ManualPurchaseItem{
			{Description: "LEITE ITALAC INT TP 1L", Quantity: decimal.NewFromInt(2), Unit: "UN", UnitPrice: decimal.RequireFromString("4.89"), Category: "laticínios/ovos"},
			{Description: "CAFE PILAO TRAD VACUO 500G", Quantity: decimal.NewFromInt(1), Unit: "UN", UnitPrice: decimal.RequireFromString("18.90"), Category: "mercearia"},
			{Description: "BANANA PRATA KG", Quantity: decimal.RequireFromString("0.80"), Unit: "KG", UnitPrice: decimal.RequireFromString("5.99"), Category: "hortifruti"},
		},
	}
	return s.ManualDraft(ctx, householdID, in)
}

func (s *PurchaseService) GetDocument(ctx context.Context, householdID, id uuid.UUID) (domain.FiscalDocument, error) {
	doc, err := s.repo.GetFiscalDocument(ctx, householdID, id)
	if err != nil {
		return doc, err
	}
	products, err := s.repo.ListProducts(ctx, householdID)
	if err != nil {
		return doc, err
	}
	for i := range doc.Items {
		cands := nfce.RankProducts(doc.Items[i].DescriptionNormalized, products)
		doc.Items[i].AmbiguousCandidates = cands
	}
	return doc, nil
}

func (s *PurchaseService) Confirm(ctx context.Context, householdID, userID uuid.UUID, in domain.ConfirmPurchaseInput) (domain.FiscalDocument, []domain.Movement, error) {
	if in.DocumentID == uuid.Nil {
		return domain.FiscalDocument{}, nil, fmt.Errorf("documentId é obrigatório")
	}
	if len(in.Items) == 0 {
		return domain.FiscalDocument{}, nil, fmt.Errorf("envie a lista de itens confirmados")
	}

	tx, err := s.repo.Pool().Begin(ctx)
	if err != nil {
		return domain.FiscalDocument{}, nil, err
	}
	defer tx.Rollback(ctx)

	doc, err := s.repo.LockFiscalForConfirm(ctx, tx, householdID, in.DocumentID)
	if err != nil {
		return domain.FiscalDocument{}, nil, err
	}
	if doc.Status == domain.FiscalConfirmed {
		return domain.FiscalDocument{}, nil, fmt.Errorf("documento já confirmado — mesma chave não republica estoque")
	}
	if doc.AccessKey != "" {
		// Double-check unique confirmed key (race-safe with lock + unique index)
		other, err := s.repo.FindFiscalByAccessKey(ctx, householdID, doc.AccessKey)
		if err == nil && other.ID != doc.ID && other.Status == domain.FiscalConfirmed {
			return domain.FiscalDocument{}, nil, fmt.Errorf("já existe entrada confirmada para esta chave de acesso")
		}
	}

	byID := make(map[uuid.UUID]domain.ConfirmPurchaseItem, len(in.Items))
	for _, it := range in.Items {
		byID[it.ItemID] = it
	}

	dbItems, err := s.repo.ListFiscalItems(ctx, doc.ID)
	if err != nil {
		return domain.FiscalDocument{}, nil, err
	}

	movements := make([]domain.Movement, 0)
	includedCount := 0

	for _, dbItem := range dbItems {
		choice, ok := byID[dbItem.ID]
		if !ok {
			// default: keep included flag from draft
			choice = domain.ConfirmPurchaseItem{
				ItemID:      dbItem.ID,
				Included:    dbItem.Included,
				Description: dbItem.Description,
				Quantity:    dbItem.Quantity,
				Unit:        dbItem.Unit,
				Category:    dbItem.Category,
				ProductID:   dbItem.MatchedProductID,
				CreateNew:   dbItem.CreateNew,
			}
		}
		if !choice.Included {
			continue
		}
		includedCount++

		desc := strings.TrimSpace(choice.Description)
		if desc == "" {
			desc = dbItem.Description
		}
		qty := choice.Quantity
		if qty.IsZero() {
			qty = dbItem.Quantity
		}
		if qty.IsNegative() || qty.IsZero() {
			return domain.FiscalDocument{}, nil, fmt.Errorf("quantidade inválida em %s", desc)
		}
		unit := normalizeUnit(choice.Unit)
		if unit == "" {
			unit = dbItem.Unit
		}
		category := choice.Category
		if category == "" {
			category = dbItem.Category
		}

		// Ambiguidade: se havia candidatos e o usuário não escolheu productId nem createNew, bloqueia.
		if dbItem.MatchStatus == domain.MatchAmbiguous && choice.ProductID == nil && !choice.CreateNew {
			return domain.FiscalDocument{}, nil, fmt.Errorf(
				"item ambíguo «%s»: escolha o produto correspondente ou marque criar novo",
				desc,
			)
		}

		normalized := domain.NormalizeDescription(desc)
		var product domain.Product

		if choice.ProductID != nil {
			product, err = getProductForUpdate(ctx, tx, householdID, *choice.ProductID)
			if err != nil {
				return domain.FiscalDocument{}, nil, fmt.Errorf("produto selecionado inválido: %w", err)
			}
			newQty := product.Quantity.Add(qty)
			if err := s.repo.UpdateProductBalance(ctx, tx, product.ID, newQty, domain.StatusConfirmed); err != nil {
				return domain.FiscalDocument{}, nil, err
			}
			product.Quantity = newQty
			product.ConfirmationStatus = domain.StatusConfirmed
		} else {
			existing, err := s.repo.FindProductByNormalized(ctx, tx, householdID, normalized)
			if err == repository.ErrNotFound {
				product = domain.Product{
					ID:                    uuid.New(),
					HouseholdID:           householdID,
					Description:           desc,
					DescriptionNormalized: normalized,
					Category:              category,
					Unit:                  unit,
					Quantity:              qty,
					ConfirmationStatus:    domain.StatusConfirmed,
				}
				product, err = s.repo.InsertProduct(ctx, tx, product)
				if err != nil {
					return domain.FiscalDocument{}, nil, err
				}
			} else if err != nil {
				return domain.FiscalDocument{}, nil, err
			} else {
				product = existing
				newQty := existing.Quantity.Add(qty)
				if err := s.repo.UpdateProductBalance(ctx, tx, product.ID, newQty, domain.StatusConfirmed); err != nil {
					return domain.FiscalDocument{}, nil, err
				}
				product.Quantity = newQty
				product.ConfirmationStatus = domain.StatusConfirmed
			}
		}

		origin := fmt.Sprintf("nfce:%s", doc.ID.String())
		if doc.AccessKey != "" {
			origin = fmt.Sprintf("nfce:%s", doc.AccessKey)
		}
		movement := domain.Movement{
			ID:            uuid.New(),
			HouseholdID:   householdID,
			ProductID:     product.ID,
			MovementType:  domain.MovementPurchaseEntry,
			QuantityDelta: qty,
			BalanceAfter:  product.Quantity,
			Origin:        origin,
			Notes:         fmt.Sprintf("NFC-e %s · %s", doc.UF, doc.IssuerName),
			IsConsumption: false,
			CreatedBy:     &userID,
		}
		movement, err = s.repo.InsertMovement(ctx, tx, movement)
		if err != nil {
			return domain.FiscalDocument{}, nil, err
		}
		movement.ProductName = product.Description
		movements = append(movements, movement)

		if err := s.repo.UpdateFiscalItemConfirmation(ctx, tx, dbItem.ID, product.ID, movement.ID, true); err != nil {
			return domain.FiscalDocument{}, nil, err
		}
	}

	if includedCount == 0 {
		return domain.FiscalDocument{}, nil, fmt.Errorf("selecione ao menos um item para entrar no estoque")
	}

	if err := s.repo.MarkFiscalConfirmed(ctx, tx, doc.ID); err != nil {
		return domain.FiscalDocument{}, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.FiscalDocument{}, nil, err
	}

	full, err := s.repo.GetFiscalDocument(ctx, householdID, doc.ID)
	if err != nil {
		return domain.FiscalDocument{}, nil, err
	}
	full.AlreadyPosted = true
	return full, movements, nil
}

func normalizeUnit(u string) string {
	u = strings.TrimSpace(strings.ToLower(u))
	switch u {
	case "", "un", "und", "unid", "unidade":
		return "un"
	case "kg", "kilo", "kilograma":
		return "kg"
	case "g", "gr", "grama", "gramas":
		return "g"
	case "l", "lt", "litro", "litros":
		return "l"
	case "ml":
		return "ml"
	default:
		return u
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
