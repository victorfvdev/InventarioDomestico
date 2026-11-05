package service

import (
	"context"
	"os"
	"testing"

	"github.com/estoque/api/internal/db"
	"github.com/estoque/api/internal/domain"
	"github.com/estoque/api/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func testRepo(t *testing.T) (*repository.Repository, domain.SessionInfo) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://estoque:estoque@127.0.0.1:5432/estoque?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := repository.New(pool)
	session, err := repo.EnsureBootstrap(ctx, "Casa Teste Consumo", "Tester")
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	return repo, session
}

func TestConsumption_InterpretConfirmUndo(t *testing.T) {
	repo, session := testRepo(t)
	ctx := context.Background()
	inv := NewInventoryService(repo)
	svc := NewConsumptionService(repo)

	unique := uuid.New().String()[:8]
	banana, _, err := inv.ManualEntry(ctx, session.Household.ID, session.User.ID, domain.ManualEntryInput{
		Description: "Banana domínio " + unique,
		Category:    "hortifruti",
		Unit:        "un",
		Quantity:    decimal.NewFromInt(10),
	})
	if err != nil {
		t.Fatalf("seed banana: %v", err)
	}

	result, err := svc.Interpret(ctx, session.Household.ID, "Comi duas bananas.")
	if err != nil {
		t.Fatalf("interpret: %v", err)
	}
	if !result.Mocked || result.Provider != "local-ptbr-parser" {
		t.Fatalf("expected local mock parser, got %+v", result)
	}
	if len(result.Interpretation.Lines) != 1 {
		t.Fatalf("lines: %+v", result.Interpretation.Lines)
	}
	line := result.Interpretation.Lines[0]
	if line.Unmatched {
		t.Fatalf("expected at least one banana match in inventory, got %+v", line)
	}
	if !line.Quantity.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("qty=%s", line.Quantity)
	}

	before := banana.Quantity
	movements, err := svc.Confirm(ctx, session.Household.ID, session.User.ID, domain.ConfirmConsumptionInput{
		InterpretationID: result.Interpretation.ID,
		Items: []domain.ConfirmConsumptionItem{{
			TempID:    line.TempID,
			Included:  true,
			ProductID: banana.ID, // human confirms which product
			Quantity:  decimal.NewFromInt(2),
			Unit:      "un",
		}},
	})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if len(movements) != 1 || !movements[0].IsConsumption || movements[0].MovementType != domain.MovementConsumption {
		t.Fatalf("movement: %+v", movements)
	}

	updated, err := repo.GetProduct(ctx, session.Household.ID, banana.ID)
	if err != nil {
		t.Fatalf("get product: %v", err)
	}
	wantAfter := before.Sub(decimal.NewFromInt(2))
	if !updated.Quantity.Equal(wantAfter) {
		t.Fatalf("balance after consume=%s want %s", updated.Quantity, wantAfter)
	}

	undoMov, err := svc.Undo(ctx, session.Household.ID, session.User.ID, movements[0].ID)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if undoMov.IsConsumption || undoMov.MovementType != domain.MovementInventoryAdjust {
		t.Fatalf("undo must be adjust≠consumption: %+v", undoMov)
	}
	restored, err := repo.GetProduct(ctx, session.Household.ID, banana.ID)
	if err != nil {
		t.Fatalf("get restored: %v", err)
	}
	if !restored.Quantity.Equal(before) {
		t.Fatalf("balance after undo=%s want %s", restored.Quantity, before)
	}
}

func TestConsumption_DoesNotInventUnmatched(t *testing.T) {
	repo, session := testRepo(t)
	ctx := context.Background()
	svc := NewConsumptionService(repo)

	result, err := svc.Interpret(ctx, session.Household.ID, "Comi iogurte grego importado")
	if err != nil {
		t.Fatalf("interpret: %v", err)
	}
	if len(result.Interpretation.Lines) != 1 {
		t.Fatalf("lines: %+v", result.Interpretation.Lines)
	}
	line := result.Interpretation.Lines[0]
	if !line.Unmatched || line.Included {
		t.Fatalf("unmatched line should not be auto-included: %+v", line)
	}

	_, err = svc.Confirm(ctx, session.Household.ID, session.User.ID, domain.ConfirmConsumptionInput{
		InterpretationID: result.Interpretation.ID,
		Items: []domain.ConfirmConsumptionItem{{
			TempID:    line.TempID,
			Included:  true,
			ProductID: uuid.Nil,
			Quantity:  decimal.NewFromInt(1),
		}},
	})
	if err == nil {
		t.Fatal("confirm without product should fail")
	}
}

func TestConsumption_QuickRequiresConfirm(t *testing.T) {
	repo, session := testRepo(t)
	ctx := context.Background()
	inv := NewInventoryService(repo)
	svc := NewConsumptionService(repo)
	p, _, err := inv.ManualEntry(ctx, session.Household.ID, session.User.ID, domain.ManualEntryInput{
		Description: "Leite teste quick",
		Category:    "laticínios",
		Unit:        "un",
		Quantity:    decimal.NewFromInt(5),
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err = svc.Quick(ctx, session.Household.ID, session.User.ID, domain.QuickConsumeInput{
		ProductID: p.ID,
		Quantity:  decimal.NewFromInt(1),
		Confirm:   false,
	})
	if err == nil {
		t.Fatal("quick without confirm must fail")
	}
	mov, err := svc.Quick(ctx, session.Household.ID, session.User.ID, domain.QuickConsumeInput{
		ProductID: p.ID,
		Quantity:  decimal.NewFromInt(1),
		Confirm:   true,
	})
	if err != nil {
		t.Fatalf("quick: %v", err)
	}
	if !mov.IsConsumption {
		t.Fatalf("expected consumption: %+v", mov)
	}
}
