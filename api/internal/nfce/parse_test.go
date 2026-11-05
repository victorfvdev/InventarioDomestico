package nfce

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractAccessKeyFromQR(t *testing.T) {
	url := "https://www.sefaz.mt.gov.br/nfce/consultanfce?p=51241103887073000156650010001234561123456780|2|1|1|A1B2C3D4"
	key, err := ExtractAccessKey(url)
	if err != nil {
		t.Fatal(err)
	}
	if key != FixtureAccessKey {
		t.Fatalf("got %s", key)
	}
	if !IsMatoGrosso(key) {
		t.Fatal("expected MT")
	}
	if ModelFromAccessKey(key) != "65" {
		t.Fatalf("model=%s", ModelFromAccessKey(key))
	}
}

func TestParseFixtureHTML(t *testing.T) {
	path := filepath.Join("fixture", "cuiaba-atacadao.html")
	html, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ParseHTML(string(html))
	if err != nil {
		t.Fatal(err)
	}
	if doc.AccessKey != FixtureAccessKey {
		t.Fatalf("access key=%s", doc.AccessKey)
	}
	if doc.IssuerName == "" {
		t.Fatal("missing issuer")
	}
	if len(doc.Items) < 5 {
		t.Fatalf("items=%d", len(doc.Items))
	}
	if doc.Items[0].Description == "" || doc.Items[0].Quantity.IsZero() {
		t.Fatalf("first item invalid: %+v", doc.Items[0])
	}
}

func TestScoreMatch(t *testing.T) {
	if ScoreMatch("leite italac int tp 1l", "leite italac int tp 1l") != 1 {
		t.Fatal("exact")
	}
	if ScoreMatch("cafe pilao", "cafe pilao trad vacuo 500g") < 0.5 {
		t.Fatal("partial")
	}
	if ScoreMatch("banana", "arroz tio joao") >= 0.35 {
		t.Fatal("should not match")
	}
}
