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
		// simple_table_top_pat is ANCHORED ("=+( +=+)+ *$"), and these
		// are why that matters: prose with two "=" signs in it has two
		// runs of "=" separated by spaces, which is all the column
		// scanner looks at. 68 real-world corpus files contain a line
		// like the first one, and each drew a "Malformed table." ERROR
		// the moment a failed table attempt stopped being silent.
		"we use u = Unicode object and s = Python string": false,
		"a = b":          false,
		"=====  =====  ": true, // trailing spaces are allowed
		"=====  ===== x": false,
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

// TestMalformedSimpleTable covers text in the MARGIN between two
// columns, which docutils refuses the whole table for:
// TableMarkupError("Text in column margin in table line N.").
//
// This parser sliced each column and dropped whatever fell between
// them, so a tab expanded across the gap took a whole cell with it:
// "a<TAB>b       c" became a two-cell row and the "b" was simply GONE.
// That is why this half was worth its own round -- the grid half was a
// missing diagnostic, this one was missing CONTENT.
//
// The LAST column is exempt on purpose: text past its right edge
// extends it, which is a documented docutils feature and is what
// extendLastColumnForOverflow already did.
func TestMalformedSimpleTable(t *testing.T) {
	t.Run("text between two columns is malformed", func(t *testing.T) {
		got := doctree.Dump(Parse("========  ========\na       b       c\n========  ========\nd         e\n========  ========\n"))
		if !strings.Contains(got, "Text in column margin in table line 2.") {
			t.Fatalf("no margin error:\n%s", got)
		}
		if strings.Contains(got, "<table>") {
			t.Errorf("a malformed table was still built:\n%s", got)
		}
		// The block is quoted AS WRITTEN: the borders are "=" here, and
		// the parser rewrites them to "-" internally, on a copy.
		if !strings.Contains(got, "========  ========") {
			t.Errorf("the block was not quoted as the author wrote it:\n%s", got)
		}
		// And the content that used to vanish is in the quoted block.
		if !strings.Contains(got, "a       b       c") {
			t.Errorf("the row's own text did not survive:\n%s", got)
		}
	})
	t.Run("text past the LAST column extends it", func(t *testing.T) {
		// The control: this is legal, and must stay a table.
		got := doctree.Dump(Parse("========  ========\na         b that runs on\n========  ========\n"))
		if strings.Contains(got, "Malformed") {
			t.Errorf("an overflowing LAST column is legal:\n%s", got)
		}
		if !strings.Contains(got, "<table>") {
			t.Errorf("expected a table:\n%s", got)
		}
	})
	t.Run("a well-formed table is untouched", func(t *testing.T) {
		got := doctree.Dump(Parse("========  ========\na         b\n========  ========\n\nnext\n"))
		if strings.Contains(got, "Malformed") || strings.Contains(got, "Blank line required") {
			t.Errorf("a good table drew a diagnostic:\n%s", got)
		}
	})

	// The four cases below are isolate_simple_table's own failure exits
	// (states.py, read directly), which this package used to leave
	// silent: two of them degraded to a paragraph with no diagnostic at
	// all, and the third BUILT a table the reference refuses. The first
	// is the docutils testsuite's own test_SimpleTableParser.py[5], whose
	// recorded expectation is a TableMarkupError because that file
	// exercises tableparser directly -- so it was never judged here until
	// the corpus gained a document-level expectation for it (v0.136.8).
	// Every dump below is the reference's own output for that input.
	t.Run("a border of a different width than the top border", func(t *testing.T) {
		got := doctree.Dump(Parse("=======  =====  ======\nA bad table     cell 2\ncell 3          cell 4\n============  ======\n"))
		want := "<document>\n" +
			"    <system_message level=\"3\" line=\"4\" type=\"ERROR\">\n" +
			"        <paragraph>\n" +
			"            Malformed table.\n" +
			"            Bottom border or header rule does not match top border.\n" +
			"        <literal_block>\n" +
			"            =======  =====  ======\n" +
			"            A bad table     cell 2\n" +
			"            cell 3          cell 4\n" +
			"            ============  ======\n"
		if strings.TrimRight(got, "\n") != strings.TrimRight(want, "\n") {
			t.Errorf("dump =\n%s\nwant:\n%s", got, want)
		}
	})
	t.Run("no bottom border before the end of the input", func(t *testing.T) {
		got := doctree.Dump(Parse("======  ======\na       b\n"))
		if !strings.Contains(got, "No bottom table border found.") {
			t.Errorf("no diagnostic:\n%s", got)
		}
		if strings.Contains(got, "or no blank line") {
			t.Errorf("the wrong half of the message: no border was found at all:\n%s", got)
		}
	})
	t.Run("a bottom border with no blank line after it is NOT a table", func(t *testing.T) {
		got := doctree.Dump(Parse("======  ======\na       b\n======  ======\ntext\n"))
		if !strings.Contains(got, "No bottom table border found or no blank line after table bottom.") {
			t.Errorf("no diagnostic:\n%s", got)
		}
		// This is the half that USED to build a table: the border was
		// found, so the old code took it as success.
		if strings.Contains(got, "<table>") {
			t.Errorf("a table was built where the reference refuses one:\n%s", got)
		}
		// And what followed it is still parsed as itself.
		if !strings.Contains(got, "Blank line required after table.") || !strings.Contains(got, "\n        text\n") {
			t.Errorf("the trailing text or its warning is missing:\n%s", got)
		}
	})
	t.Run("CONTROL: prose with two = signs is still a paragraph", func(t *testing.T) {
		got := doctree.Dump(Parse("- In examples we use u = Unicode object and s = Python string\n"))
		if strings.Contains(got, "Malformed") {
			t.Errorf("prose was read as a table top border:\n%s", got)
		}
	})
}
