package rst

import "testing"

func TestIsGridTableTopLine(t *testing.T) {
	cases := map[string]bool{
		"+-----+-----+": true,
		"+-----------+": true,
		"+--+":          true,
		"+-+":           false, // too short (< 4 chars)
		"+++":           false,
		"----+-----+":   false, // doesn't start with '+'
		"+-----+-----":  false, // doesn't end with '+'
		"+=====+=====+": false, // '=' chars, not '-': that's a head/body separator, not a border
		"":              false,
	}
	for in, want := range cases {
		if got := isGridTableTopLine(in); got != want {
			t.Errorf("isGridTableTopLine(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsGridTableHeadSepLine(t *testing.T) {
	cases := map[string]bool{
		"+=====+=====+": true,
		"+-----+-----+": false, // '-' chars: that's a border, not a head/body separator
		"+==+":          true,
		"+=+":           false,
		"":              false,
	}
	for in, want := range cases {
		if got := isGridTableHeadSepLine(in); got != want {
			t.Errorf("isGridTableHeadSepLine(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsolateGridTable(t *testing.T) {
	lines := []string{
		"+-----+-----+",
		"| a   | b   |",
		"+-----+-----+",
		"| 1   | 2   |",
		"+-----+-----+",
		"",
		"Not part of the table.",
	}
	block, next, ok := isolateGridTable(lines, 0)
	if !ok {
		t.Fatal("isolateGridTable failed to match a well-formed table")
	}
	if len(block) != 5 {
		t.Errorf("isolateGridTable block has %d lines, want 5: %v", len(block), block)
	}
	if next != 5 {
		t.Errorf("isolateGridTable next = %d, want 5", next)
	}
}

func TestIsolateGridTableRejectsUnclosedTable(t *testing.T) {
	lines := []string{
		"+-----+-----+",
		"| a   | b   |",
		"no border and no closing row at all",
	}
	if _, _, ok := isolateGridTable(lines, 0); ok {
		t.Fatal("isolateGridTable matched a table with no valid bottom border")
	}
}

// TestTryParseGridTableRejectsMultipleHeadSeps exercises the "more than
// one head/body separator" rejection: docutils raises a
// TableMarkupError for this; this parser just doesn't recognize the
// table at all (see gridtable.go's SCOPE note on diagnostics).
func TestTryParseGridTableRejectsMultipleHeadSeps(t *testing.T) {
	p := &parser{}
	lines := []string{
		"+-----+-----+",
		"| a   | b   |",
		"+=====+=====+",
		"| c   | d   |",
		"+=====+=====+",
		"| e   | f   |",
		"+-----+-----+",
	}
	if _, _, ok := p.tryParseGridTable(lines, 0); ok {
		t.Fatal("tryParseGridTable matched a table with two head/body separators")
	}
}

func TestDedentCellLines(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"strips common one-space left padding and right padding", []string{" Header row  ", " b  "}, []string{"Header row", "b"}},
		{"blank lines don't affect the common-indent calculation", []string{"  a", "", "  b"}, []string{"a", "", "b"}},
		{"a genuinely nested line keeps its relative indent", []string{"term", "  body"}, []string{"term", "  body"}},
		{"no leading whitespace at all: no-op besides right-trim", []string{"a  ", "b"}, []string{"a", "b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dedentCellLines(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("dedentCellLines(%v) = %v, want %v", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("dedentCellLines(%v)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
				}
			}
		})
	}
}
