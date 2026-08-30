package rst

import "testing"

func TestIsSimpleTableTopLine(t *testing.T) {
	cases := map[string]bool{
		"=====  =====": true,
		"=====":        false, // only one group: not a valid top border
		"":             false,
		"not a border": false,
	}
	for in, want := range cases {
		if got := isSimpleTableTopLine(in); got != want {
			t.Errorf("isSimpleTableTopLine(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsSimpleTableBorderLine(t *testing.T) {
	cases := map[string]bool{
		"=====  =====": true,
		"=====":        true,
		"":             false,
		"-----":        false,
		"= not":        false,
	}
	for in, want := range cases {
		if got := isSimpleTableBorderLine(in); got != want {
			t.Errorf("isSimpleTableBorderLine(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsSpanLine(t *testing.T) {
	cases := map[string]bool{
		"------------": true,
		"--  --":       true,
		"":             false,
		"=====":        false,
		"- not":        false,
	}
	for in, want := range cases {
		if got := isSpanLine(in); got != want {
			t.Errorf("isSpanLine(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseColumnsChar(t *testing.T) {
	cols := parseColumnsChar("=====  =====", '=', nil, 0)
	want := []tableColumn{{0, 5}, {7, 12}}
	if len(cols) != len(want) {
		t.Fatalf("parseColumnsChar returned %d columns, want %d: %v", len(cols), len(want), cols)
	}
	for i, c := range cols {
		if c != want[i] {
			t.Errorf("column %d = %v, want %v", i, c, want[i])
		}
	}

	// A span whose last column doesn't reach the table's right border
	// is malformed and rejected.
	canonical := []tableColumn{{0, 5}, {7, 12}}
	if got := parseColumnsChar("--", '-', canonical, 12); got != nil {
		t.Errorf("parseColumnsChar with incomplete span = %v, want nil", got)
	}
}

// TestTryParseSimpleTableRejectsNonTable confirms a line that isn't a
// valid top border is simply declined, letting the caller fall back to
// ordinary block parsing (a paragraph in this case).
func TestTryParseSimpleTableRejectsNonTable(t *testing.T) {
	p := &parser{}
	lines := []string{"not a table top line"}
	if _, _, ok := p.tryParseSimpleTable(lines, 0); ok {
		t.Fatal("tryParseSimpleTable matched a non-table line")
	}
}
