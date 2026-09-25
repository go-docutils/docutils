package rst

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestCSVCellDiagnosticLine pins the line a diagnostic raised inside a
// csv-table cell carries: the directive's content OFFSET — one less than
// the first content line's own number — plus the line index WITHIN THE
// CELL.
//
// The ROW does not enter into it, which is the part that has to be
// measured rather than assumed: docutils hands each cell to nested_parse
// as its own StringList starting at zero, so two cells in different rows
// report the SAME line while two lines of one quoted cell report
// different ones. Every expectation below came from wrapping
// Reporter.system_message on the reference and printing the line it was
// handed.
//
// This package passed -1 as the cell's line base, so every diagnostic in
// every csv table carried no line at all.
func TestCSVCellDiagnosticLine(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   int
	}{
		{
			"a role in the first row",
			"Para.\n\n.. csv-table::\n   :header: A, B\n\n   :nosuch:`x`, b\n   c, d\n", 5,
		},
		{
			// The SECOND row reports the same line as the first: the row
			// index is not part of it.
			"a role in the second row",
			"Para.\n\n.. csv-table::\n   :header: A, B\n\n   a, b\n   :nosuch:`x`, d\n", 5,
		},
		{
			// The second line of one QUOTED cell does move, by one.
			"a role on a quoted cell's second line",
			"Para.\n\n.. csv-table::\n\n   a, \"\n   :nosuch:`x`\n   \"\n", 5,
		},
		{
			// CONTROL: no option block, so the content starts one line
			// earlier and the number follows it.
			"no option block",
			"Para.\n\n.. csv-table::\n\n   :nosuch:`x`, b\n", 4,
		},
		{
			// CONTROL: the number is absolute, not table-relative.
			"further down the document",
			"Para.\n\nx\n\ny\n\n.. csv-table::\n\n   :nosuch:`x`, b\n", 8,
		},
	}
	re := regexp.MustCompile(`<system_message[^>]*line="(\d+)"`)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dump := doctree.Dump(Parse(tc.source))
			ms := re.FindAllStringSubmatch(dump, -1)
			if len(ms) == 0 {
				t.Fatalf("Parse(%q) raised no message carrying a line:\n%s", tc.source, dump)
			}
			for _, m := range ms {
				got, _ := strconv.Atoi(m[1])
				if got != tc.want {
					t.Errorf("Parse(%q): message line = %d, want %d\n%s", tc.source, got, tc.want, dump)
				}
			}
		})
	}
}
