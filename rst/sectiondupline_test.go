package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestDuplicateSectionNameLine pins where a duplicate SECTION name is
// reported: against the title's UNDERLINE -- the last line of the title
// construct, which is where docutils' state machine sits once it has
// read the whole thing.
//
// Nothing recorded a line for a section at all before v0.91.0, so every
// such message came out with no line attribute. That was 83 real-world
// files, sphinx's entire changelog set among them (each release repeats
// "Bugs fixed"), and zero docutils testsuite fixtures.
//
// All five shapes were run against the reference.
func TestDuplicateSectionNameLine(t *testing.T) {
	cases := []struct {
		name, source, wantLine string
	}{
		{
			"underlined, duplicate title on line 6",
			"A\n=\n\ntext\n\nA\n=\n\nmore\n", `line="7"`,
		},
		{
			"underlined, duplicate title on line 4",
			"A\n=\n\nA\n=\n\nmore\n", `line="5"`,
		},
		{
			// OVERLINED: still the underline, not the overline and not
			// the title text -- the two differ by two lines here, so this
			// case alone rules out both other readings.
			"overlined, duplicate title on line 8",
			"=\nA\n=\n\ntext\n\n=\nA\n=\n\nmore\n", `line="9"`,
		},
		{
			"nothing follows the duplicate",
			"A\n=\n\ntext\n\nA\n=\n", `line="7"`,
		},
		{
			"a nested subsection",
			"T\n=\n\nA\n-\n\ntext\n\nA\n-\n\nx\n", `line="10"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !strings.Contains(got, "Duplicate implicit target name") {
				t.Fatalf("no duplicate-name message at all:\n%s", got)
			}
			if !strings.Contains(got, tc.wantLine) {
				t.Errorf("missing %s:\n%s", tc.wantLine, got)
			}
		})
	}
}
