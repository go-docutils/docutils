package rst

import (
	"reflect"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

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
	if _, _, ok := p.tryParseSimpleTable(lines, 0, 0); ok {
		t.Fatal("tryParseSimpleTable matched a non-table line")
	}
}

// TestTableCellCarriesRealLines covers the line a diagnostic raised
// inside a table CELL reports. Both table parsers passed
// parseBlockLines the -1 "unknown" sentinel, so every such message came
// out with no line attribute -- the last two of the three callers still
// doing that after v0.113.0 wired up footnote bodies.
//
// Every cell in a row needs a DIFFERENT base, which is why this could
// never have been one value per table:
//
//   - grid: a cell's content is block rows top+1..bottom-1, and block[r]
//     is lines[i+r], so the base is i+top+1+lineBase.
//   - simple: cell.lineOffset is already the row's first line within
//     block, so the base is i+lineOffset+lineBase -- no border row to
//     skip, since a simple table's row slice starts on the text.
//
// The expectations come from running real docutils. The second case is
// the one that makes the rule specific: a second paragraph inside one
// cell reports its own line, two rows below the cell's first.
func TestTableCellCarriesRealLines(t *testing.T) {
	cases := []struct {
		name, source string
		want         []string
	}{
		{
			"a grid cell",
			"intro\n\n" +
				"+--------------+-------+\n" +
				"| a            | b     |\n" +
				"+==============+=======+\n" +
				"| :pep:`x550y` | d     |\n" +
				"+--------------+-------+\n",
			[]string{"6"},
		},
		{
			"a second paragraph inside one grid cell",
			"intro\n\n" +
				"+--------------+---+\n" +
				"| a            | b |\n" +
				"+==============+===+\n" +
				"| one          | d |\n" +
				"|              |   |\n" +
				"| :pep:`x550y` |   |\n" +
				"+--------------+---+\n",
			[]string{"8"},
		},
		{
			"a simple-table cell",
			"intro\n\n" +
				"============  =====\n" +
				"a             b\n" +
				"============  =====\n" +
				":pep:`x550y`  d\n" +
				"============  =====\n",
			[]string{"6"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := systemMessageLines(tc.source)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("system_message lines = %q, want %q\n%s", got, tc.want, doctree.Dump(Parse(tc.source)))
			}
		})
	}
}
