package nfce

import (
	"strings"

	"github.com/estoque/api/internal/domain"
)

// ScoreMatch returns 0..1 similarity between a note line and a product description.
func ScoreMatch(noteNorm, productNorm string) float64 {
	if noteNorm == "" || productNorm == "" {
		return 0
	}
	if noteNorm == productNorm {
		return 1
	}
	if strings.Contains(noteNorm, productNorm) || strings.Contains(productNorm, noteNorm) {
		return 0.85
	}
	noteTokens := tokenSet(noteNorm)
	prodTokens := tokenSet(productNorm)
	if len(noteTokens) == 0 || len(prodTokens) == 0 {
		return 0
	}
	inter := 0
	for t := range noteTokens {
		if prodTokens[t] {
			inter++
		}
	}
	union := len(noteTokens) + len(prodTokens) - inter
	if union == 0 {
		return 0
	}
	jaccard := float64(inter) / float64(union)
	// Boost when significant tokens overlap (brand/product words > 2 chars)
	if inter >= 2 && jaccard >= 0.4 {
		return clamp(0.55 + jaccard*0.4)
	}
	return jaccard
}

func RankProducts(noteNorm string, products []domain.Product) []domain.ProductMatch {
	var out []domain.ProductMatch
	for _, p := range products {
		score := ScoreMatch(noteNorm, p.DescriptionNormalized)
		if score < 0.35 {
			continue
		}
		out = append(out, domain.ProductMatch{
			ProductID:   p.ID,
			Description: p.Description,
			Score:       score,
		})
	}
	// simple insertion sort by score desc
	for i := 1; i < len(out); i++ {
		j := i
		for j > 0 && out[j-1].Score < out[j].Score {
			out[j-1], out[j] = out[j], out[j-1]
			j--
		}
	}
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

func DecideMatch(candidates []domain.ProductMatch) (domain.MatchStatus, *domain.ProductMatch, float64) {
	if len(candidates) == 0 {
		return domain.MatchNew, nil, 0
	}
	best := candidates[0]
	if best.Score >= 0.98 {
		return domain.MatchExact, &best, best.Score
	}
	if len(candidates) >= 2 && candidates[1].Score >= 0.55 && (best.Score-candidates[1].Score) < 0.12 {
		return domain.MatchAmbiguous, &best, best.Score
	}
	if best.Score >= 0.55 {
		return domain.MatchSuggested, &best, best.Score
	}
	return domain.MatchNew, nil, best.Score
}

func GuessCategory(desc string) string {
	d := strings.ToLower(desc)
	switch {
	case containsAny(d, "leite", "queijo", "iogurte", "ovo"):
		return "laticínios/ovos"
	case containsAny(d, "arroz", "feij", "cafe", "café", "acucar", "açúcar"):
		return "mercearia"
	case containsAny(d, "banana", "maca", "maçã", "fruta", "alface", "tomate"):
		return "hortifruti"
	case containsAny(d, "saco plast", "sacola"):
		return "outros"
	default:
		return "geral"
	}
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

func tokenSet(s string) map[string]bool {
	parts := strings.Fields(s)
	out := make(map[string]bool, len(parts))
	for _, p := range parts {
		if len(p) <= 2 {
			continue
		}
		// skip noisy commercial tokens
		switch p {
		case "tp", "pct", "cxl", "trad", "int", "vacuo", "kg", "un", "gd":
			continue
		}
		out[p] = true
	}
	return out
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
