package consumption

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/estoque/api/internal/domain"
	"github.com/shopspring/decimal"
)

// ParsedMention is one product mention extracted from natural-language PT-BR text.
// The parser never invents ingredients that were not mentioned.
type ParsedMention struct {
	RawPhrase         string
	ProductHint       string
	Quantity          decimal.Decimal
	Unit              string
	QuantityInferred  bool
	Approx            bool
}

var (
	wordNumbers = map[string]int{
		"um": 1, "uma": 1, "uns": 1, "umas": 1,
		"dois": 2, "duas": 2,
		"tres": 3, "três": 3,
		"quatro": 4,
		"cinco": 5,
		"seis": 6,
		"sete": 7,
		"oito": 8,
		"nove": 9,
		"dez": 10,
		"meia": 1, // meia dúzia handled separately
		"meio": 1, // meio litro etc. with unit
	}

	unitAliases = map[string]string{
		"un": "un", "und": "un", "unidade": "un", "unidades": "un",
		"ml": "ml", "mililitro": "ml", "mililitros": "ml",
		"l": "l", "lt": "l", "litro": "l", "litros": "l",
		"g": "g", "gr": "g", "grama": "g", "gramas": "g",
		"kg": "kg", "quilo": "kg", "quilos": "kg",
		"cx": "cx", "caixa": "cx", "caixas": "cx",
		"pct": "pct", "pacote": "pct", "pacotes": "pct",
		"lata": "lata", "latas": "lata",
		"duzia": "dz", "dúzia": "dz", "duzias": "dz", "dúzias": "dz",
		"xicara": "xícara", "xícara": "xícara", "xicaras": "xícara", "xícaras": "xícara",
		"colher": "colher", "colheres": "colher",
	}

	// Verbs / fillers stripped from the start of a clause.
	leadVerbs = regexp.MustCompile(`(?i)^(comi|usei|utilizei|utilizei|usei|consumi|tomei|bebi|preparei|fiz|gastei|precisei\s+de|peguei)\s+`)

	approxWord = regexp.MustCompile(`(?i)\b(aproximadamente|aprox\.?|cerca\s+de|uns|umas|mais\s+ou\s+menos)\b`)

	// Split clauses on connectors while keeping each consumption mention separate.
	clauseSplit = regexp.MustCompile(`(?i)\s*(?:,|;|\be\b|\bdepois\b|\btamb[eé]m\b|\bmais\b)\s+`)

	qtyUnitProduct = regexp.MustCompile(`(?i)^\s*(?P<qty>\d+(?:[.,]\d+)?|[a-zà-ú]+)?\s*(?P<unit>ml|mililitros?|litro?s?|lt|l|kg|quilos?|g|gramas?|cx|caixas?|pct|pacotes?|lata?s?|un(?:idade)?s?|und|d[uú]zias?|x[ií]caras?|colheres?)?\s*(?:de\s+)?(?P<product>.+?)\s*$`)
)

// ParseText extracts product mentions from a PT-BR consumption sentence.
func ParseText(text string) []ParsedMention {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	text = strings.TrimRight(text, ".!?")

	clauses := splitClauses(text)
	var out []ParsedMention
	seen := map[string]bool{}

	for _, clause := range clauses {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}
		m := parseClause(clause)
		if m == nil || strings.TrimSpace(m.ProductHint) == "" {
			continue
		}
		key := domain.NormalizeDescription(m.ProductHint) + "|" + m.Quantity.String() + "|" + m.Unit
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, *m)
	}
	return out
}

func splitClauses(text string) []string {
	// Prefer splitting only when the connector introduces another verb-led consumption.
	parts := clauseSplit.Split(text, -1)
	if len(parts) == 1 {
		return parts
	}
	var rebuilt []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		rebuilt = append(rebuilt, p)
	}
	return rebuilt
}

func parseClause(clause string) *ParsedMention {
	approx := approxWord.MatchString(clause)
	cleaned := approxWord.ReplaceAllString(clause, " ")
	cleaned = leadVerbs.ReplaceAllString(cleaned, "")
	cleaned = strings.TrimSpace(cleaned)
	cleaned = collapseSpaces(cleaned)
	if cleaned == "" {
		return nil
	}

	// Special: "meia dúzia de ovos"
	if m := parseMeiaDuzia(cleaned); m != nil {
		m.Approx = approx
		m.RawPhrase = clause
		return m
	}

	// Special: "ovos mexidos" / recipes still only extract explicitly named products
	if m := parseKnownPatterns(cleaned); m != nil {
		m.Approx = approx
		m.RawPhrase = clause
		return m
	}

	match := qtyUnitProduct.FindStringSubmatch(cleaned)
	if match == nil {
		// bare product → quantity 1 inferred
		hint := stripTrailingNoise(cleaned)
		if hint == "" {
			return nil
		}
		return &ParsedMention{
			RawPhrase:        clause,
			ProductHint:      hint,
			Quantity:         decimal.NewFromInt(1),
			Unit:             "un",
			QuantityInferred: true,
			Approx:           approx,
		}
	}

	groups := subexpMap(qtyUnitProduct, match)
	qtyStr := strings.TrimSpace(groups["qty"])
	unitStr := strings.ToLower(strings.TrimSpace(groups["unit"]))
	product := stripTrailingNoise(strings.TrimSpace(groups["product"]))

	qty := decimal.NewFromInt(1)
	inferred := true
	if qtyStr != "" {
		if n, ok := parseQuantityToken(qtyStr); ok {
			qty = n
			inferred = false
		}
	}

	unit := "un"
	if unitStr != "" {
		if u, ok := unitAliases[unitStr]; ok {
			unit = u
		} else {
			unit = unitStr
		}
		// "meio litro" style: word "meio" + unit
		if strings.EqualFold(qtyStr, "meio") || strings.EqualFold(qtyStr, "meia") {
			qty = decimal.NewFromFloat(0.5)
			inferred = false
		}
	}

	if product == "" {
		return nil
	}

	// If product still starts with a unit word ("de leite"), strip leftover "de"
	product = strings.TrimPrefix(product, "de ")
	product = strings.TrimSpace(product)

	return &ParsedMention{
		RawPhrase:        clause,
		ProductHint:      product,
		Quantity:         qty,
		Unit:             unit,
		QuantityInferred: inferred,
		Approx:           approx,
	}
}

func parseMeiaDuzia(s string) *ParsedMention {
	re := regexp.MustCompile(`(?i)^meia\s+d[uú]zia\s+(?:de\s+)?(.+)$`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	return &ParsedMention{
		ProductHint:      stripTrailingNoise(m[1]),
		Quantity:         decimal.NewFromInt(6),
		Unit:             "un",
		QuantityInferred: false,
	}
}

func parseKnownPatterns(s string) *ParsedMention {
	lower := strings.ToLower(s)

	// "três ovos mexidos" / "2 ovos mexidos" — product is ovos, not invent other ingredients
	reOvos := regexp.MustCompile(`(?i)^(\d+(?:[.,]\d+)?|[a-zà-ú]+)?\s*ovos?\b(.*)$`)
	if m := reOvos.FindStringSubmatch(lower); m != nil && (strings.Contains(m[2], "mexid") || strings.TrimSpace(m[2]) == "" || strings.HasPrefix(strings.TrimSpace(m[2]), " ")) {
		qty := decimal.NewFromInt(1)
		inferred := true
		if m[1] != "" {
			if n, ok := parseQuantityToken(m[1]); ok {
				qty = n
				inferred = false
			}
		}
		return &ParsedMention{
			ProductHint:      "ovos",
			Quantity:         qty,
			Unit:             "un",
			QuantityInferred: inferred,
		}
	}

	reBanana := regexp.MustCompile(`(?i)^(\d+(?:[.,]\d+)?|[a-zà-ú]+)?\s*bananas?\b`)
	if m := reBanana.FindStringSubmatch(lower); m != nil {
		qty := decimal.NewFromInt(1)
		inferred := true
		if m[1] != "" {
			if n, ok := parseQuantityToken(m[1]); ok {
				qty = n
				inferred = false
			}
		}
		return &ParsedMention{
			ProductHint:      "banana",
			Quantity:         qty,
			Unit:             "un",
			QuantityInferred: inferred,
		}
	}

	return nil
}

func parseQuantityToken(tok string) (decimal.Decimal, bool) {
	tok = strings.TrimSpace(strings.ToLower(tok))
	tok = strings.ReplaceAll(tok, ",", ".")
	if n, ok := wordNumbers[tok]; ok {
		return decimal.NewFromInt(int64(n)), true
	}
	f, err := strconv.ParseFloat(tok, 64)
	if err != nil || f < 0 {
		return decimal.Zero, false
	}
	return decimal.NewFromFloat(f), true
}

func subexpMap(re *regexp.Regexp, match []string) map[string]string {
	out := map[string]string{}
	for i, name := range re.SubexpNames() {
		if i == 0 || name == "" || i >= len(match) {
			continue
		}
		out[name] = match[i]
	}
	return out
}

func stripTrailingNoise(s string) string {
	s = strings.TrimSpace(s)
	// drop trailing "mexidos", "cozidos" style adjectives only when product already clear —
	// keep them out of the match hint for ovos via known patterns; here just trim punctuation.
	s = strings.Trim(s, " .,;:")
	return s
}

func collapseSpaces(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
