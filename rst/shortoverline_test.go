package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestShortOverlineOnlyOnFailure pins where the "so short" INFO belongs.
// A short overline is NOT wrong by itself: real docutils' Line.text and
// Line.underline reach short_overline only from inside their FAILURE
// branches, so "===\nOne\n===" is an ordinary section title. Checking
// the length FIRST -- which this parser used to do -- annotated every
// well-formed short title and refused to build it, and the corpus
// fixture that caught it is the one whose own prose describes the
// work-around this threshold comes from.
//
// Every case below was run against the reference, including the two
// LONG-overline controls: at four characters and up the same three
// failures produce a WARNING or an ERROR instead, which is what makes
// this a threshold rather than a blanket rule.
func TestShortOverlineOnlyOnFailure(t *testing.T) {
	cases := []struct {
		name        string
		source      string
		wantSection bool
		wantMessage string
	}{
		{
			"a three-character overline matching its title is an ordinary section",
			"===\nOne\n===\n\ntext\n", true, "",
		},
		{
			"two characters is fine too, when the title fits",
			"--\nHi\n--\n\ntext\n", true, "",
		},
		{
			"short AND too narrow for its title: demoted to text",
			"==\nTitle\n==\n\ntext\n", false, "Possible incomplete section title.",
		},
		{
			"short with a mismatched underline: demoted, not an ERROR",
			"===\nOne\n---\n\ntext\n", false, "Possible incomplete section title.",
		},
		{
			"short with no underline at all: demoted, not an ERROR",
			"===\nOne\ntext here\n", false, "Possible incomplete section title.",
		},
		{
			// The control that makes this a threshold: at four
			// characters the SAME shape is a real title plus a warning.
			"long and too narrow is a section WITH a warning",
			"====\nA longer title\n====\n\ntext\n", true, "Title overline too short.",
		},
		{
			// And the mismatch stays an ERROR at four characters.
			"long with a mismatched underline is still an ERROR",
			"=====\nOne\n-----\n\ntext\n", false, "Title overline & underline mismatch.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if hasSection := strings.Contains(got, "<section"); hasSection != tc.wantSection {
				t.Errorf("section built = %v, want %v:\n%s", hasSection, tc.wantSection, got)
			}
			switch tc.wantMessage {
			case "":
				if strings.Contains(got, "system_message") {
					t.Errorf("a well-formed title drew a diagnostic:\n%s", got)
				}
			default:
				if !strings.Contains(got, tc.wantMessage) {
					t.Errorf("missing %q:\n%s", tc.wantMessage, got)
				}
			}
		})
	}
}
