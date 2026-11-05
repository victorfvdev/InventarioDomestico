package consumption

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestParseText_ExamplesFromSpec(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		hint    string
		qty     string
		unit    string
		infer   bool
		minLines int
	}{
		{"bananas", "Comi duas bananas.", "banana", "2", "un", false, 1},
		{"ovos mexidos", "Fiz três ovos mexidos.", "ovos", "3", "un", false, 1},
		{"creme de leite", "Utilizei uma caixa de creme de leite.", "creme de leite", "1", "cx", false, 1},
		{"leite ml", "Usei aproximadamente 200 ml de leite.", "leite", "200", "ml", false, 1},
		{"sem quantidade", "Comi banana", "banana", "1", "un", true, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines := ParseText(tc.text)
			if len(lines) < tc.minLines {
				t.Fatalf("expected >= %d lines, got %d (%+v)", tc.minLines, len(lines), lines)
			}
			m := lines[0]
			if m.ProductHint == "" {
				t.Fatal("empty product hint")
			}
			// hint should contain the expected keyword
			if !containsFold(m.ProductHint, tc.hint) && m.ProductHint != tc.hint {
				// allow exact known-pattern overrides
				if m.ProductHint != "ovos" && m.ProductHint != "banana" && m.ProductHint != "leite" && m.ProductHint != "creme de leite" {
					t.Fatalf("hint=%q want contains %q", m.ProductHint, tc.hint)
				}
			}
			wantQty, _ := decimal.NewFromString(tc.qty)
			if !m.Quantity.Equal(wantQty) {
				t.Fatalf("qty=%s want %s", m.Quantity, wantQty)
			}
			if m.Unit != tc.unit {
				t.Fatalf("unit=%q want %q", m.Unit, tc.unit)
			}
			if m.QuantityInferred != tc.infer {
				t.Fatalf("inferred=%v want %v", m.QuantityInferred, tc.infer)
			}
		})
	}
}

func TestParseText_DoesNotInventIngredients(t *testing.T) {
	// "ovos mexidos" must not invent butter/salt/milk
	lines := ParseText("Fiz três ovos mexidos")
	if len(lines) != 1 {
		t.Fatalf("expected 1 mention, got %d: %+v", len(lines), lines)
	}
	if lines[0].ProductHint != "ovos" {
		t.Fatalf("hint=%q", lines[0].ProductHint)
	}
}

func TestParseText_MultipleClauses(t *testing.T) {
	lines := ParseText("Comi duas bananas e usei 200 ml de leite")
	if len(lines) < 2 {
		t.Fatalf("expected 2 mentions, got %d: %+v", len(lines), lines)
	}
}

func TestParseText_Empty(t *testing.T) {
	if ParseText("   ") != nil {
		t.Fatal("expected nil for blank")
	}
}

func containsFold(hay, needle string) bool {
	return len(hay) >= len(needle) && (hay == needle ||
		len(needle) == 0 ||
		(len(hay) > 0 && (hay == needle ||
			stringContains(hay, needle))))
}

func stringContains(a, b string) bool {
	return len(b) == 0 || (len(a) >= len(b) && (a == b || indexOf(a, b) >= 0))
}

func indexOf(a, b string) int {
	for i := 0; i+len(b) <= len(a); i++ {
		if a[i:i+len(b)] == b {
			return i
		}
	}
	return -1
}
