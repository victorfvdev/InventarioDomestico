package consumption

import (
	"strings"

	"github.com/estoque/api/internal/domain"
	"github.com/estoque/api/internal/nfce"
)

// Alias hints help PT-BR plurals/common names match inventory descriptions.
var productAliases = map[string][]string{
	"banana":        {"banana", "bananas", "banana prata", "banana nanica"},
	"bananas":       {"banana", "bananas"},
	"ovo":           {"ovo", "ovos"},
	"ovos":          {"ovo", "ovos"},
	"leite":         {"leite", "leite integral", "leite desnatado", "leite uht"},
	"creme de leite": {"creme de leite", "creme leite"},
	"arroz":         {"arroz", "arroz branco", "arroz tipo 1"},
	"feijao":        {"feijao", "feijão", "feijao carioca", "feijão carioca"},
	"cafe":          {"cafe", "café", "cafe torrado"},
	"acucar":        {"acucar", "açúcar", "acucar cristal"},
	"pao":           {"pao", "pão", "pao de forma", "pão de forma"},
	"manteiga":      {"manteiga"},
	"queijo":        {"queijo", "queijo mussarela", "mussarela"},
	"tomate":        {"tomate", "tomates"},
	"cebola":        {"cebola", "cebolas"},
	"alho":          {"alho"},
	"oleo":          {"oleo", "óleo", "oleo de soja"},
	"agua":          {"agua", "água", "agua mineral"},
}

// RankForMention scores inventory products against a parsed product hint.
func RankForMention(hint string, products []domain.Product) []domain.ProductMatch {
	norm := domain.NormalizeDescription(hint)
	candidates := map[string]domain.ProductMatch{}

	add := func(score float64, p domain.Product) {
		if score < 0.35 {
			return
		}
		id := p.ID.String()
		if prev, ok := candidates[id]; ok && prev.Score >= score {
			return
		}
		candidates[id] = domain.ProductMatch{
			ProductID:   p.ID,
			Description: p.Description,
			Score:       score,
		}
	}

	for _, p := range products {
		add(nfce.ScoreMatch(norm, p.DescriptionNormalized), p)
		pn := p.DescriptionNormalized
		if norm == "" || pn == "" {
			continue
		}
		// Prefer head-token matches ("leite" → "leite integral") over mid-phrase
		// containment ("leite" → "creme de leite").
		if pn == norm {
			add(1, p)
		} else if strings.HasPrefix(pn, norm+" ") || strings.HasPrefix(norm, pn+" ") {
			add(0.93, p)
		} else if firstToken(pn) == norm || firstToken(norm) == firstToken(pn) {
			add(0.9, p)
		} else if strings.Contains(pn, " "+norm) || strings.Contains(pn, norm+" ") {
			add(0.62, p)
		}
	}

	if aliases, ok := productAliases[norm]; ok {
		for _, alias := range aliases {
			an := domain.NormalizeDescription(alias)
			for _, p := range products {
				add(nfce.ScoreMatch(an, p.DescriptionNormalized), p)
				if an != "" && strings.Contains(p.DescriptionNormalized, an) {
					add(0.9, p)
				}
			}
		}
	}
	// also try singular/plural heuristics
	for _, variant := range pluralVariants(norm) {
		for _, p := range products {
			add(nfce.ScoreMatch(variant, p.DescriptionNormalized), p)
			if strings.Contains(p.DescriptionNormalized, variant) {
				add(0.86, p)
			}
		}
	}

	out := make([]domain.ProductMatch, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c)
	}
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

func firstToken(s string) string {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func pluralVariants(norm string) []string {
	if norm == "" {
		return nil
	}
	var out []string
	if strings.HasSuffix(norm, "oes") && len(norm) > 3 {
		out = append(out, strings.TrimSuffix(norm, "oes")+"ao")
	}
	if strings.HasSuffix(norm, "as") && len(norm) > 2 {
		out = append(out, strings.TrimSuffix(norm, "s"))
	}
	if strings.HasSuffix(norm, "es") && len(norm) > 2 {
		out = append(out, strings.TrimSuffix(norm, "es"), strings.TrimSuffix(norm, "s"))
	}
	if strings.HasSuffix(norm, "s") && len(norm) > 1 {
		out = append(out, strings.TrimSuffix(norm, "s"))
	} else {
		out = append(out, norm+"s", norm+"es")
	}
	return out
}
