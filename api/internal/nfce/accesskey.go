package nfce

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var digitsOnly = regexp.MustCompile(`[^0-9]+`)

// ExtractAccessKey pulls a 44-digit NFC-e access key from raw text or a QR URL.
func ExtractAccessKey(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("informe a chave de acesso ou a URL do QR Code")
	}

	if strings.Contains(raw, "http://") || strings.Contains(raw, "https://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("URL do QR Code inválida")
		}
		// SEFAZ-MT típico: ?p=CHAVE|2|1|...
		if p := u.Query().Get("p"); p != "" {
			part := strings.Split(p, "|")[0]
			return normalizeKey(part)
		}
		if chNFe := u.Query().Get("chNFe"); chNFe != "" {
			return normalizeKey(chNFe)
		}
		// fallback: qualquer sequência de 44 dígitos na URL
		digits := digitsOnly.ReplaceAllString(raw, "")
		if len(digits) >= 44 {
			return normalizeKey(digits[:44])
		}
		return "", fmt.Errorf("não encontrei chave de 44 dígitos na URL do QR")
	}

	return normalizeKey(raw)
}

func normalizeKey(s string) (string, error) {
	key := digitsOnly.ReplaceAllString(s, "")
	if len(key) != 44 {
		return "", fmt.Errorf("chave de acesso deve ter 44 dígitos (recebido %d)", len(key))
	}
	return key, nil
}

// UFFromAccessKey returns the IBGE UF code embedded in positions 0-1.
func UFFromAccessKey(key string) string {
	if len(key) < 2 {
		return ""
	}
	return key[:2]
}

// IsMatoGrosso reports whether the key belongs to UF MT (código 51).
func IsMatoGrosso(key string) bool {
	return UFFromAccessKey(key) == "51"
}

// ModelFromAccessKey returns the document model (55=NF-e, 65=NFC-e).
func ModelFromAccessKey(key string) string {
	if len(key) < 22 {
		return ""
	}
	return key[20:22]
}
