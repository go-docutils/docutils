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
	block, next, _, _, ok := isolateGridTable(lines, 0)
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
	if _, _, _, _, ok := isolateGridTable(lines, 0); ok {
		t.Fatal("isolateGridTable matched a table with no valid bottom border")
	}
}

// TestTryParseGridTableRejectsMultipleHeadSeps exercises the "more than
// one head/body separator" case. It used to assert that this parser did
// not recognize the table AT ALL, which was the documented scope gap:
// docutils raises a TableMarkupError, and Body.table turns that into a
// "Malformed table." ERROR quoting the block. Since v0.136.8 that
// message is produced here too, so the assertion is the message rather
// than the refusal -- refusing quietly and reporting are exactly what
// this case has to tell apart, and the whole dump is compared against the
// reference's own output for this input.
func TestTryParseGridTableRejectsMultipleHeadSeps(t *testing.T) {
	const src = "+-----+-----+\n| a   | b   |\n+=====+=====+\n| c   | d   |\n+=====+=====+\n| e   | f   |\n+-----+-----+\n"
	want := "<document>\n" +
		"    <system_message level=\"3\" line=\"5\" type=\"ERROR\">\n" +
		"        <paragraph>\n" +
		"            Malformed table.\n" +
		"            Multiple head/body row separators (table lines 3 and 5); only one allowed.\n" +
		"        <literal_block>\n" +
		"            +-----+-----+\n" +
		"            | a   | b   |\n" +
		"            +=====+=====+\n" +
		"            | c   | d   |\n" +
		"            +=====+=====+\n" +
		"            | e   | f   |\n" +
		"            +-----+-----+\n"
	got := doctree.Dump(Parse(src))
	if strings.TrimRight(got, "\n") != strings.TrimRight(want, "\n") {
		t.Errorf("dump =\n%s\nwant:\n%s", got, want)
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

// TestColumnWidth covers docutils' utils.column_width, which is what a
// title underline's length is compared against (states.py:2888 and
// :3147). This package had it as "count the runes that are not
// unicode.Mn", which is wrong twice over.
//
// It never doubled East Asian Wide characters, so a CJK title got no
// "underline too short" warning it had earned. And unicode.Mn is not
// the set unicodedata.combining tests: they disagree on 1127 code
// points -- 1089 are in Mn with a combining class of ZERO, 38 combine
// without being in Mn. Measured, not guessed, which is the only reason
// the ranges in eastasian.go are generated rather than approximated.
//
// The last case is the one a three-way switch gets wrong: U+3099 is
// Wide AND combining, and docutils sums the widths then subtracts one
// per combining character, so it is 2-1 = 1, not 0.
func TestColumnWidth(t *testing.T) {
	for _, tc := range []struct {
		s    string
		want int
	}{
		{"", 0},
		{"abc", 3},
		{"café", 4},   // precomposed: 4 code points, 4 columns
		{"café", 4},  // decomposed: 5 code points, 4 columns
		{"中文", 4},     // two Wide characters
		{"Ti中tle", 7}, // the probe's own case
		{"Ａ", 2},      // FULLWIDTH LATIN CAPITAL A
		{"…", 1},      // an ellipsis is narrow
		{"゙", 1},      // Wide AND combining: 2-1
		{"中゙", 3},     // 2 + (2-1)
	} {
		if got := ColumnWidth(tc.s); got != tc.want {
			t.Errorf("ColumnWidth(%q) = %d, want %d", tc.s, got, tc.want)
		}
	}
}

// TestTitleUnderlineUsesColumnWidth pins the parser-level consequence:
// a title of five code points but seven COLUMNS needs seven underline
// characters, not five.
func TestTitleUnderlineUsesColumnWidth(t *testing.T) {
	const short = "Ti中tle\n======\n\nbody\n" // 6 dashes under 7 columns
	const ok = "Ti中tle\n=======\n\nbody\n"   // 7
	if got := doctree.Dump(Parse(short)); !strings.Contains(got, "Title underline too short") {
		t.Errorf("a 6-wide underline under a 7-column title raised no warning:\n%s", got)
	}
	if got := doctree.Dump(Parse(ok)); strings.Contains(got, "Title underline too short") {
		t.Errorf("a 7-wide underline under a 7-column title raised a warning:\n%s", got)
	}
	// The control: ASCII is unaffected either way.
	if got := doctree.Dump(Parse("Title\n=====\n\nbody\n")); strings.Contains(got, "too short") {
		t.Errorf("an exact ASCII underline raised a warning:\n%s", got)
	}
}

// TestMalformedGridTable covers what a grid table that does not FORM a
// table produces. Once the top border matches, docutils has committed
// to a table: any failure is "Malformed table." plus a detail, with the
// offending block quoted as a literal_block. This parser reported
// nothing at all and let the lines fall back to ordinary block parsing,
// so a broken table silently became a paragraph.
//
// Every expectation was read off real docutils, including the LINE each
// message carries, which differs per failure and is the fiddly part.
func TestMalformedGridTable(t *testing.T) {
	for _, tc := range []struct{ name, source, detail, line string }{
		{
			// A row wider than its border.
			"right border not aligned",
			"+--------+--------+\n| a     b    | c      |\n+========+========+\n| d      | e      |\n+--------+--------+\n",
			"Right border not aligned or missing.", "2",
		},
		{
			// A row that does not start with "+" or "|" truncates the
			// block, and what is left has no bottom border.
			"bottom border missing",
			"+--------+--------+\n| a\nb    | c      |\n+========+========+\n| d      | e      |\n+--------+--------+\n",
			"Bottom border missing or corrupt.", "3",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !strings.Contains(got, "Malformed table.\n") {
				t.Fatalf("no Malformed table. error:\n%s", got)
			}
			if !strings.Contains(got, tc.detail) {
				t.Errorf("detail %q missing:\n%s", tc.detail, got)
			}
			if !strings.Contains(got, `line="`+tc.line+`"`) {
				t.Errorf("want line %s:\n%s", tc.line, got)
			}
			// The block itself must be quoted, or the reader cannot see
			// WHICH table failed.
			if !strings.Contains(got, "<literal_block>") {
				t.Errorf("the offending block was not quoted:\n%s", got)
			}
		})
	}
	t.Run("a well-formed table is untouched", func(t *testing.T) {
		got := doctree.Dump(Parse("+---+---+\n| a | b |\n+---+---+\n\nnext\n"))
		if strings.Contains(got, "Malformed") || strings.Contains(got, "Blank line required") {
			t.Errorf("a good table drew a diagnostic:\n%s", got)
		}
	})
	t.Run("a table not followed by a blank line warns", func(t *testing.T) {
		// docutils raises this from table_top, the CALLER of the table
		// parser, so it applies to a well-formed table too -- which is
		// why neither path had it here.
		got := doctree.Dump(Parse("+---+---+\n| a | b |\n+---+---+\nnext\n"))
		if !strings.Contains(got, "Blank line required after table.") {
			t.Errorf("no warning:\n%s", got)
		}
		if !strings.Contains(got, `line="4"`) {
			t.Errorf("want line 4:\n%s", got)
		}
	})
}

// TestGridTableParseIncomplete is the docutils testsuite's own
// test_TableParser.py[grid_tables][8]: a table whose cells are not
// rectangles. GridTableParser.parse_table ends with check_parse_complete
// and raises TableMarkupError when the cell scan has not accounted for
// the whole block, which Body.table turns into "Malformed table." with
// that detail. This package returned "not a table" instead, so the whole
// thing came out as a paragraph and nothing said why -- and because the
// testsuite records an EXCEPTION for this case (it exercises tableparser
// directly), it was never judged here until the corpus gained a
// document-level expectation for it (v0.136.8).
//
// The second case is the CONTROL: the scan must still complete for a
// table whose cells DO span, or "report when incomplete" would quietly
// become "report whenever a cell is not a plain rectangle".
func TestGridTableParseIncomplete(t *testing.T) {
	const src = "+--------------+-------------+\n| A bad table. |             |\n+--------------+             |\n| Cells must be rectangles.  |\n+----------------------------+\n"
	want := "<document>\n" +
		"    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n" +
		"        <paragraph>\n" +
		"            Malformed table.\n" +
		"            Malformed table; parse incomplete.\n" +
		"        <literal_block>\n" +
		"            +--------------+-------------+\n" +
		"            | A bad table. |             |\n" +
		"            +--------------+             |\n" +
		"            | Cells must be rectangles.  |\n" +
		"            +----------------------------+\n"
	if got := doctree.Dump(Parse(src)); strings.TrimRight(got, "\n") != strings.TrimRight(want, "\n") {
		t.Errorf("dump =\n%s\nwant:\n%s", got, want)
	}
	// A legal column span over the same shape.
	spanned := "+--------------+-------------+\n| a            | b           |\n+--------------+-------------+\n| a real span                |\n+----------------------------+\n"
	got := doctree.Dump(Parse(spanned))
	if strings.Contains(got, "Malformed") {
		t.Errorf("a legal span was reported as malformed:\n%s", got)
	}
	if !strings.Contains(got, "morecols=\"1\"") {
		t.Errorf("the span did not survive:\n%s", got)
	}
}
