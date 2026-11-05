package nfce

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FixtureAccessKey is the well-known key embedded in the offline MT fixture.
const FixtureAccessKey = "51241103887073000156650010001234561123456780"

// SEFAZ limits (documentados para o operador):
//
//   - Consulta pública da NFC-e em MT exige navegação com sessão/cookies e, com
//     frequência, CAPTCHA — automação headless costuma ser bloqueada.
//   - O HTML retornado muda com o tempo; o parser tolera o layout típico da
//     consulta (emitente + tabela de itens), não um schema estável de API.
//   - Indisponibilidade, manutenção e rate-limit da SEFAZ-MT são comuns;
//     por isso o Estoque sempre oferece fixture offline + entrada manual.
//   - URL pública típica (pode mudar):
//     https://www.sefaz.mt.gov.br/nfce/consultanfce?p={chave}|2|1|1|{hash}
const DefaultConsultBase = "https://www.sefaz.mt.gov.br/nfce/consultanfce"

type FetchResult struct {
	Source      string // fixture | sefaz | qr
	HTML        string
	Parsed      ParsedDocument
	Notes       string
	LiveTried   bool
	LiveOK      bool
	LiveError   string
	UsedFixture bool
}

type Adapter struct {
	HTTPClient   *http.Client
	ConsultBase  string
	FixturePath  string
	ForceFixture bool
}

func NewAdapter(fixturePath string, forceFixture bool) *Adapter {
	return &Adapter{
		HTTPClient: &http.Client{
			Timeout: 12 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("muitos redirects na SEFAZ")
				}
				return nil
			},
		},
		ConsultBase:  DefaultConsultBase,
		FixturePath:  fixturePath,
		ForceFixture: forceFixture,
	}
}

// FetchByAccessKey tries SEFAZ-MT (unless forced/fixture key) and falls back to fixture.
func (a *Adapter) FetchByAccessKey(ctx context.Context, accessKey string, preferFixture bool) (FetchResult, error) {
	key, err := ExtractAccessKey(accessKey)
	if err != nil {
		return FetchResult{}, err
	}
	if !IsMatoGrosso(key) {
		return FetchResult{}, fmt.Errorf(
			"esta chave não é de Mato Grosso (UF IBGE %s; esperado 51). O Estoque prioriza NFC-e MT/Cuiabá",
			UFFromAccessKey(key),
		)
	}
	if mod := ModelFromAccessKey(key); mod != "" && mod != "65" {
		return FetchResult{}, fmt.Errorf("modelo do documento é %s (esperado 65 = NFC-e)", mod)
	}

	result := FetchResult{}
	skipLive := preferFixture || a.ForceFixture || key == FixtureAccessKey

	if !skipLive {
		result.LiveTried = true
		html, liveErr := a.fetchLive(ctx, key)
		if liveErr == nil && looksLikeNFCeHTML(html) {
			parsed, parseErr := ParseHTML(html)
			if parseErr == nil {
				if parsed.AccessKey == "" {
					parsed.AccessKey = key
				}
				result.Source = "sefaz"
				result.HTML = html
				result.Parsed = parsed
				result.LiveOK = true
				result.Notes = "Consulta SEFAZ-MT retornou HTML parseável."
				return result, nil
			}
			liveErr = parseErr
		}
		if liveErr != nil {
			result.LiveError = liveErr.Error()
		} else {
			result.LiveError = "HTML da SEFAZ sem tabela de itens (possível captcha/login)"
		}
	}

	html, err := a.loadFixture()
	if err != nil {
		if result.LiveError != "" {
			return result, fmt.Errorf("SEFAZ falhou (%s) e fixture também: %w", result.LiveError, err)
		}
		return result, err
	}
	parsed, err := ParseHTML(html)
	if err != nil {
		return result, err
	}

	// Canonical fixture key always; for demos of the known key keep it.
	// If live failed for another MT key, still load fixture items but stamp the requested key
	// only when the caller explicitly asked for fixture mode — otherwise keep fixture key
	// and explain the fallback (avoids confirming a fake note under a real key).
	if key == FixtureAccessKey || preferFixture || a.ForceFixture {
		parsed.AccessKey = key
	} else if parsed.AccessKey == "" {
		parsed.AccessKey = FixtureAccessKey
	}

	result.Source = "fixture"
	result.UsedFixture = true
	result.HTML = html
	result.Parsed = parsed
	if result.LiveTried {
		result.Notes = "SEFAZ-MT indisponível ou bloqueou automação (" + result.LiveError +
			"). Usando fixture offline MT — confira se a chave é a da fixture de demo."
	} else {
		result.Notes = "Fixture offline de NFC-e MT (Cuiabá / Atacadão) — sem chamada à SEFAZ."
	}
	return result, nil
}

func (a *Adapter) fetchLive(ctx context.Context, key string) (string, error) {
	url := fmt.Sprintf("%s?p=%s|2|1|1|DEMO", strings.TrimRight(a.ConsultBase, "/"), key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "EstoqueDomestico/0.1 (+local; NFC-e MT consult)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	res, err := a.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("rede SEFAZ-MT: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode >= 400 {
		return "", fmt.Errorf("SEFAZ-MT HTTP %d", res.StatusCode)
	}
	return string(body), nil
}

func (a *Adapter) loadFixture() (string, error) {
	path := a.FixturePath
	if path == "" {
		path = DefaultFixturePath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("ler fixture NFC-e: %w", err)
	}
	return string(data), nil
}

func DefaultFixturePath() string {
	candidates := []string{
		filepath.Join("internal", "nfce", "fixture", "cuiaba-atacadao.html"),
		filepath.Join("api", "internal", "nfce", "fixture", "cuiaba-atacadao.html"),
		filepath.Join("..", "internal", "nfce", "fixture", "cuiaba-atacadao.html"),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return candidates[0]
}

func looksLikeNFCeHTML(html string) bool {
	lower := strings.ToLower(html)
	if strings.Contains(lower, "captcha") && !strings.Contains(lower, "tabresult") {
		return false
	}
	return strings.Contains(lower, "chave") || strings.Contains(lower, "nfce") || strings.Contains(lower, "nfc-e")
}
