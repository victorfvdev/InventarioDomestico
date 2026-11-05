package nfce

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// ParsedDocument is the normalized result of an NFC-e HTML/JSON payload.
type ParsedDocument struct {
	AccessKey  string
	UF         string
	IssuerName string
	IssuerCNPJ string
	IssuedAt   *time.Time
	Total      decimal.Decimal
	Items      []ParsedItem
	RawNotes   string
}

type ParsedItem struct {
	LineNumber  int
	Description string
	Quantity    decimal.Decimal
	Unit        string
	UnitPrice   decimal.Decimal
	TotalPrice  decimal.Decimal
	EAN         string
}

var (
	reChaveDigits = regexp.MustCompile(`(?i)chave[^0-9]*([0-9][0-9\s.]{40,}[0-9])`)
	reCNPJ        = regexp.MustCompile(`(?i)CNPJ[^0-9]*([0-9]{2}\.?[0-9]{3}\.?[0-9]{3}/?[0-9]{4}-?[0-9]{2})`)
	reEmitName    = regexp.MustCompile(`(?is)Nome\s*/\s*Raz[aã]o Social</th>\s*<td[^>]*>([^<]+)`)
	reEmitAlt     = regexp.MustCompile(`(?is)<td[^>]*>\s*(ATACADAO[^<]*|SUPERMERCADO[^<]*|HIPER[^<]*|MERCADO[^<]*)\s*</td>`)
	reIssued      = regexp.MustCompile(`(?i)Data de Emiss[aã]o</th>\s*<td[^>]*>\s*([0-9]{2}/[0-9]{2}/[0-9]{4}(?:\s+[0-9]{2}:[0-9]{2}:[0-9]{2})?)`)
	reTotal       = regexp.MustCompile(`(?i)Valor Total</th>\s*<td[^>]*>\s*(?:R\$\s*)?([0-9]+[.,][0-9]{2})`)
	reRow         = regexp.MustCompile(`(?is)<tr>\s*<td[^>]*>\s*(\d+)\s*</td>\s*<td[^>]*>\s*([^<]+?)\s*</td>\s*<td[^>]*>\s*([0-9]+[.,][0-9]+)\s*</td>\s*<td[^>]*>\s*([A-Za-z]+)\s*</td>\s*<td[^>]*>\s*([0-9]+[.,][0-9]+)\s*</td>\s*<td[^>]*>\s*([0-9]+[.,][0-9]+)\s*</td>\s*<td[^>]*>\s*([^<]*?)\s*</td>\s*</tr>`)
)

// ParseHTML extracts emitente + itens from a SEFAZ-like NFC-e HTML page.
func ParseHTML(html string) (ParsedDocument, error) {
	var out ParsedDocument
	html = strings.TrimSpace(html)
	if html == "" {
		return out, fmt.Errorf("payload HTML vazio")
	}

	if m := reChaveDigits.FindStringSubmatch(html); len(m) > 1 {
		key, err := normalizeKey(m[1])
		if err == nil {
			out.AccessKey = key
		}
	}
	if out.AccessKey == "" {
		// try any 44-digit sequence
		compact := digitsOnly.ReplaceAllString(html, "")
		if idx := strings.Index(compact, "51"); idx >= 0 && len(compact) >= idx+44 {
			candidate := compact[idx : idx+44]
			if IsMatoGrosso(candidate) {
				out.AccessKey = candidate
			}
		}
	}

	if m := reCNPJ.FindStringSubmatch(html); len(m) > 1 {
		out.IssuerCNPJ = digitsOnly.ReplaceAllString(m[1], "")
	}
	if m := reEmitName.FindStringSubmatch(html); len(m) > 1 {
		out.IssuerName = cleanText(m[1])
	} else if m := reEmitAlt.FindStringSubmatch(html); len(m) > 1 {
		out.IssuerName = cleanText(m[1])
	}
	if m := reIssued.FindStringSubmatch(html); len(m) > 1 {
		if t, err := parseBRDate(m[1]); err == nil {
			out.IssuedAt = &t
		}
	}
	if m := reTotal.FindStringSubmatch(html); len(m) > 1 {
		out.Total = parseBRNumber(m[1])
	}

	matches := reRow.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		item := ParsedItem{
			LineNumber:  atoi(m[1]),
			Description: cleanText(m[2]),
			Quantity:    parseBRNumber(m[3]),
			Unit:        strings.ToUpper(cleanText(m[4])),
			UnitPrice:   parseBRNumber(m[5]),
			TotalPrice:  parseBRNumber(m[6]),
			EAN:         cleanText(m[7]),
		}
		if item.Description == "" {
			continue
		}
		out.Items = append(out.Items, item)
	}

	if len(out.Items) == 0 {
		return out, fmt.Errorf("nenhum item encontrado no HTML da NFC-e")
	}

	out.UF = "MT"
	if out.AccessKey != "" && IsMatoGrosso(out.AccessKey) {
		out.UF = "MT"
	}
	return out, nil
}

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.TrimSpace(s)
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return s
}

func parseBRNumber(s string) decimal.Decimal {
	s = strings.TrimSpace(s)
	// Brazilian: 1.234,56 or plain 4,89 / 4.89 / 6,0000
	if strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return d
}

func parseBRDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	layouts := []string{"02/01/2006 15:04:05", "02/01/2006"}
	var last error
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, s, time.FixedZone("AMT", -4*3600))
		if err == nil {
			return t, nil
		}
		last = err
	}
	return time.Time{}, last
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
