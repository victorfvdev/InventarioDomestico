package consumption

import (
	"testing"

	"github.com/estoque/api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestRankForMention_MatchesPlural(t *testing.T) {
	products := []domain.Product{
		{
			ID:                    uuid.New(),
			Description:           "Banana prata",
			DescriptionNormalized: "banana prata",
			Unit:                  "un",
			Quantity:              decimal.NewFromInt(10),
		},
		{
			ID:                    uuid.New(),
			Description:           "Ovos",
			DescriptionNormalized: "ovos",
			Unit:                  "un",
			Quantity:              decimal.NewFromInt(12),
		},
		{
			ID:                    uuid.New(),
			Description:           "Leite integral",
			DescriptionNormalized: "leite integral",
			Unit:                  "l",
			Quantity:              decimal.NewFromInt(2),
		},
	}

	bananas := RankForMention("bananas", products)
	if len(bananas) == 0 || bananas[0].Description != "Banana prata" {
		t.Fatalf("bananas match: %+v", bananas)
	}
	ovos := RankForMention("ovos", products)
	if len(ovos) == 0 || ovos[0].Description != "Ovos" {
		t.Fatalf("ovos match: %+v", ovos)
	}
	leite := RankForMention("leite", products)
	if len(leite) == 0 || leite[0].Description != "Leite integral" {
		t.Fatalf("leite match: %+v", leite)
	}
}

func TestRankForMention_PrefersHeadTokenOverMidPhrase(t *testing.T) {
	products := []domain.Product{
		{
			ID:                    uuid.New(),
			Description:           "Creme de leite",
			DescriptionNormalized: "creme de leite",
			Unit:                  "cx",
			Quantity:              decimal.NewFromInt(4),
		},
		{
			ID:                    uuid.New(),
			Description:           "Leite integral",
			DescriptionNormalized: "leite integral",
			Unit:                  "ml",
			Quantity:              decimal.NewFromInt(2000),
		},
	}
	leite := RankForMention("leite", products)
	if len(leite) == 0 || leite[0].Description != "Leite integral" {
		t.Fatalf("leite should prefer Leite integral, got %+v", leite)
	}
}

func TestRankForMention_NoInvent(t *testing.T) {
	products := []domain.Product{
		{
			ID:                    uuid.New(),
			Description:           "Arroz",
			DescriptionNormalized: "arroz",
			Unit:                  "kg",
			Quantity:              decimal.NewFromInt(5),
		},
	}
	cands := RankForMention("iogurte grego", products)
	if len(cands) != 0 {
		t.Fatalf("should not match unrelated product: %+v", cands)
	}
}
