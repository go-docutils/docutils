package rst

import (
	"reflect"
	"strings"
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

// TestSimpleTableMeasuresCodePointsNotBytes is v0.109.0's fix for the
// GRID table, applied to the sibling it did not touch. A simple table's
// borders are ASCII, so only the DATA rows were wrong -- but that is
// enough: extendLastColumnForOverflow compared a row's BYTE length
// against a column boundary counted in characters, so one accented
// letter anywhere in the table made its last column one wider than
// docutils says, and a whole table of them (PEP 3117 declares its types
// with mathematical symbols) made it several.
//
// docutils pads simple tables exactly as it pads grid ones --
// simple_table_top calls pad_double_width right where grid_table_top
// does -- so a wide character spans two columns here too, and the
// filler must not survive into the cell's text.
func TestSimpleTableMeasuresCodePointsNotBytes(t *testing.T) {
	// The second column's border is seven wide in every case; only the
	// data cell varies.
	table := func(cell string) string {
		return "=====  =======\n" +
			"a      b\n" +
			"=====  =======\n" +
			"one    " + cell + "\n" +
			"=====  =======\n"
	}
	cases := []struct {
		name, cell string
		wantWidths []string
		wantText   string
	}{
		{"ascii, the control", "two", []string{"5", "7"}, "two"},
		// 4 bytes, 3 code points: it FITS, and used to widen the column.
		{"an accented letter is one column", "café", []string{"5", "7"}, "café"},
		// 2 wide chars + 3 = 7 columns exactly.
		{"a wide character is two columns", "中文xxx", []string{"5", "7"}, "中文xxx"},
		// 2 wide chars + 5 = 9 columns: docutils widens the last column
		// to 9, measuring in columns rather than characters or bytes.
		{"and overflows by its WIDTH", "中文xxxxx", []string{"5", "9"}, "中文xxxxx"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(table(tc.cell)))
			var widths []string
			for _, l := range strings.Split(got, "\n") {
				if k := strings.Index(l, `colwidth="`); k >= 0 {
					r := l[k+len(`colwidth="`):]
					widths = append(widths, r[:strings.IndexByte(r, '"')])
				}
			}
			if !reflect.DeepEqual(widths, tc.wantWidths) {
				t.Errorf("colwidths = %q, want %q\n%s", widths, tc.wantWidths, got)
			}
			if !strings.Contains(got, "\n                            "+tc.wantText+"\n") {
				t.Errorf("cell text %q not found in:\n%s", tc.wantText, got)
			}
			// The filler standing in for a wide character's second
			// column must never reach the tree.
			if strings.ContainsRune(got, doubleWidthPad) {
				t.Errorf("padding character leaked into the tree:\n%q", got)
			}
		})
	}
}
