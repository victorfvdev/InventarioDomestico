package nfce

import "testing"

func TestNormalizeRejectsShortKey(t *testing.T) {
	_, err := ExtractAccessKey("12345")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRejectNonMT(t *testing.T) {
	// SP = 35
	key := "35241103887073000156650010001234561123456780"
	if IsMatoGrosso(key) {
		t.Fatal("SP key should not be MT")
	}
}
