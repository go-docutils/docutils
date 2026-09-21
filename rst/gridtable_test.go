package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

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
	if _, _, ok := p.tryParseGridTable(lines, 0, 0); ok {
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

// TestGridTableMeasuresCodePointsNotBytes covers the unit a grid
// table's geometry is measured in. Every case below was run against
// real docutils first; none of them is a guess about what it "should"
// do.
//
// A Python parser indexes lines by code point for free, so "…" is one
// column. Reading the same lines as Go strings made it three, pushing
// every later "|" out of line with its border and degrading the whole
// table to a paragraph -- no diagnostic, no partial table, just text.
//
// The fourth case is the one that keeps the fix honest. Plain code
// points would be wrong in the OTHER direction: docutils pads East
// Asian Wide/Fullwidth characters to two columns before measuring
// (StringList.pad_double_width), so a table whose rows line up by code
// point but not by width is one it REJECTS. Accepting it would trade a
// parse failure for a silent disagreement, which is worse.
func TestGridTableMeasuresCodePointsNotBytes(t *testing.T) {
	// One cell of each table is varied; everything else is identical.
	table := func(firstCell string) string {
		return "+-------+-------+\n" +
			"| " + firstCell + " | cd    |\n" +
			"+=======+=======+\n" +
			"| one   | two   |\n" +
			"+-------+-------+\n"
	}
	cases := []struct {
		name      string
		firstCell string
		wantTable bool
		wantText  string
	}{
		{"ascii, the control", "ab   ", true, "ab"},
		{"U+2026 is one column wide", "ab\u2026  ", true, "ab\u2026"},
		{"a wide character is TWO columns", "\u4e2d\u6587 ", true, "\u4e2d\u6587"},
		// Five columns of content for four columns of width: docutils
		// calls this a malformed table, so it must not become one here.
		{"wide characters counted as one are NOT a table", "\u4e2d\u6587   ", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(table(tc.firstCell)))
			if isTable := strings.Contains(got, "<table>"); isTable != tc.wantTable {
				t.Fatalf("parsed as a table = %v, want %v:\n%s", isTable, tc.wantTable, got)
			}
			if !tc.wantTable {
				return
			}
			if !strings.Contains(got, "\n                            "+tc.wantText+"\n") {
				t.Errorf("cell text %q not found in:\n%s", tc.wantText, got)
			}
			// The filler standing in for a wide character's second
			// column must never reach the tree.
			if strings.ContainsRune(got, doubleWidthPad) {
				t.Errorf("padding character leaked into the tree:\n%q", got)
			}
			// Both columns are seven wide in every accepted case; a
			// cell text that merely LOOKS right can still come from a
			// mis-measured grid.
			if n := strings.Count(got, `<colspec colwidth="7">`); n != 2 {
				t.Errorf("got %d colspecs of width 7, want 2:\n%s", n, got)
			}
		})
	}
}

// TestIsEastAsianWide spot-checks the generated table against the
// classification unicodedata gives, at the boundaries of a range rather
// than in its middle.
func TestIsEastAsianWide(t *testing.T) {
	for _, tc := range []struct {
		r    rune
		want bool
	}{
		{'a', false},
		{'\u2026', false},    // HORIZONTAL ELLIPSIS: ambiguous, not wide
		{'\u4e2d', true},     // CJK
		{'\uff21', true},     // FULLWIDTH LATIN CAPITAL A
		{'\u1100', true},     // first code point of the first range
		{'\u115f', true},     // last code point of that range
		{'\u1160', false},    // one past it
		{'\U0001f600', true}, // emoji are Wide
		{'\u00e9', false},    // é: two bytes, one column
	} {
		if got := isEastAsianWide(tc.r); got != tc.want {
			t.Errorf("isEastAsianWide(%q/U+%04X) = %v, want %v", tc.r, tc.r, got, tc.want)
		}
	}
}

// TestTableColumnWidth covers the exported metric. It is the same rule
// isEastAsianWide already encodes, stated as a width so a writer can
// pad with it; the cases are the ones that distinguish it from both
// len() and utf8.RuneCountInString.
func TestTableColumnWidth(t *testing.T) {
	for _, tc := range []struct {
		s    string
		want int
	}{
		{"", 0},
		{"abc", 3},
		{"ab…", 3}, // 5 bytes, 3 code points, 3 columns
		{"é", 1},   // 2 bytes, 1 code point, 1 column
		{"中文", 4},  // 6 bytes, 2 code points, 4 columns
		{"a中b", 4}, // mixed
		{"Ａ", 2},   // FULLWIDTH LATIN CAPITAL A
	} {
		if got := TableColumnWidth(tc.s); got != tc.want {
			t.Errorf("TableColumnWidth(%q) = %d, want %d", tc.s, got, tc.want)
		}
	}
	// The parser measures with the same rule it exports: a cell padded
	// to TableColumnWidth's answer is one the parser accepts.
	cell := "中文"
	// Seven columns between the borders: one leading space, the cell,
	// then filler.
	pad := strings.Repeat(" ", 7-1-TableColumnWidth(cell))
	src := "+-------+-------+\n| " + cell + pad + "| cd    |\n+-------+-------+\n"
	if got := doctree.Dump(Parse(src)); !strings.Contains(got, "<table>") {
		t.Errorf("a cell padded to TableColumnWidth is not accepted by the parser:\n%s", got)
	}
}
