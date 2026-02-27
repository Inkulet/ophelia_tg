package app

import "testing"

func TestParseYearRange(t *testing.T) {
	tests := []struct {
		in   string
		from int
		to   int
	}{
		{"1800-1900", 1800, 1900},
		{"1900", 1900, 1900},
		{"1990-1980", 1980, 1990},
		{"19 век", 1801, 1900},
	}
	for _, tt := range tests {
		f, to := parseYearRange(tt.in)
		if f != tt.from || to != tt.to {
			t.Fatalf("parseYearRange(%q) = %d,%d; want %d,%d", tt.in, f, to, tt.from, tt.to)
		}
	}
}

func TestTokenizeSearchArgs(t *testing.T) {
	in := `field:"точные науки" tag:математика year:1800-1900`
	toks := tokenizeSearchArgs(in)
	if len(toks) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %#v", len(toks), toks)
	}
	if toks[0] != "field:точные науки" {
		t.Fatalf("unexpected token[0]=%q", toks[0])
	}
}

func TestParseTagsText(t *testing.T) {
	tags := parseTagsText("math, physics|bio/chem")
	if len(tags) != 4 {
		t.Fatalf("expected 4 tags, got %d: %#v", len(tags), tags)
	}
}
