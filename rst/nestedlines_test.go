package rst

import (
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestNestedContentCarriesRealLines pins the absolute line numbers now
// available inside nested constructs. Every one of these used to omit the
// attribute entirely, because the content was parsed from a REBASED
// sub-slice with no known correspondence back to the document.
//
// The correspondence turns out to be exact wherever the sub-slice comes
// from consumeIndentedBlock or gatherListItemLines: both only dedent and
// trim TRAILING blanks, so entry k is the parent's line i+k. That is the
// same derivation v0.44.0 made for topic/sidebar content and v0.59.0 for
// block quotes, applied to the four remaining places it holds.
//
// Each expected line was checked against the reference implementation.
func TestNestedContentCarriesRealLines(t *testing.T) {
	cases := []struct {
		name     string
		source   string
		wantLine string
	}{
		{
			"a definition body's own diagnostic",
			"Term\n    body with *unclosed\n",
			`line="2"`,
		},
		{
			"a bullet list item's own diagnostic",
			"- item with *unclosed\n",
			`line="1"`,
		},
		{
			"a later bullet item reports its OWN line, not the list's",
			"- one\n\n- item with *unclosed\n",
			`line="3"`,
		},
		{
			"an enumerated list item's own diagnostic",
			"1. one\n\n2. item with *unclosed\n",
			`line="3"`,
		},
		{
			"a field body's own diagnostic",
			":name: value\n:other: with *unclosed\n",
			`line="2"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !contains(got, tc.wantLine) {
				t.Errorf("Parse(%q) did not report %s:\n%s", tc.source, tc.wantLine, got)
			}
		})
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
