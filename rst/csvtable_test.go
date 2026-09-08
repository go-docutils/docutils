package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestCSVTableDirective covers directives.tables.CSVTable's parse-time
// half. Every expectation is a bare Parser().parse() at the judge's
// report_level.
func TestCSVTableDirective(t *testing.T) {
	cases := []struct {
		name, source string
		want         []string
	}{
		{
			"no header option: equal column widths, all rows in the body",
			".. csv-table::\n\n   1, 2\n   3, 4\n",
			[]string{`<tgroup cols="2">`, `<colspec colwidth="50">`, "<tbody>"},
		},
		{
			":header supplies a thead the data does not",
			".. csv-table:: T\n   :header: A, B\n\n   1, 2\n",
			[]string{"<title>", "<thead>", "<tbody>"},
		},
		{
			":header-rows takes them off the DATA instead",
			".. csv-table::\n   :header-rows: 1\n\n   A, B\n   1, 2\n",
			[]string{"<thead>", "<tbody>"},
		},
		{
			// The reason this needs a CSV reader rather than a split:
			// a quoted field may contain the delimiter.
			"a quoted field keeps its comma",
			".. csv-table::\n\n   1, \"a, b\"\n",
			[]string{`<tgroup cols="2">`, "a, b"},
		},
		{
			"cells are inline-parsed",
			".. csv-table::\n\n   *em*, ``lit``\n",
			[]string{"<emphasis>", "<literal>"},
		},
		{
			// Ragged rows do not error: the column count is the MAXIMUM
			// and shorter rows are padded with empty entries.
			"ragged rows pad to the widest",
			".. csv-table::\n\n   1, 2, 3\n   4, 5\n",
			[]string{`<tgroup cols="3">`, `<colspec colwidth="33">`},
		},
		{
			":widths: overrides the equal split and marks the table",
			".. csv-table::\n   :widths: 10, 20\n\n   1, 2\n",
			[]string{`<colspec colwidth="10">`, `<colspec colwidth="20">`, `class="colwidths-given"`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("missing %q:\n%s", w, got)
				}
			}
			if strings.Contains(got, "Unknown directive type") {
				t.Errorf("a REGISTERED directive was reported as unknown:\n%s", got)
			}
		})
	}
}

// TestCSVTableUnsupportedInvocationFallsBack pins the scope boundary:
// :file:/:url: read from the filesystem or network during parsing, and
// :delim:/:quote:/:escape:/:keepspace: change the CSV dialect. None is
// ported, and such an invocation falls back to the structural
// <directive> capture rather than producing a wrong table -- the same
// fallback ".. raw::" without a format takes.
func TestCSVTableUnsupportedInvocationFallsBack(t *testing.T) {
	got := doctree.Dump(Parse(".. csv-table::\n   :file: data.csv\n"))
	if !strings.Contains(got, "<directive") {
		t.Errorf("expected the structural capture:\n%s", got)
	}
	if strings.Contains(got, "<table") {
		t.Errorf("built a table from an invocation it cannot honour:\n%s", got)
	}
}
