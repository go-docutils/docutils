package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestUnknownDirectiveKeepsTrailingBlanks pins how much blank space the
// "Unknown directive type" ERROR quotes. Body.unknown_directive quotes
// get_first_known_indented(0)'s block, which runs through EVERY blank
// line under the directive; joining that list keeps one "\n" per blank,
// and Text.pformat's own splitlines() then drops exactly ONE trailing
// empty element at PRINT time. So the visible result is n-1 blank lines
// for n blanks in the source -- and the drop belongs to the DUMP, not to
// the parser.
//
// This parser trimmed them in the parser as well, removing the same
// blank twice. 33 real-world files ride on it, because two blank lines
// before a section heading is a PEP convention; no docutils testsuite
// fixture has more than one.
//
// The counts below are the reference's own.
func TestUnknownDirectiveKeepsTrailingBlanks(t *testing.T) {
	cases := []struct {
		name       string
		source     string
		wantBlanks int
	}{
		{"one blank line leaves none behind", ".. unknown-thing:: arg\n\nAfter.\n", 0},
		{"two leave one", ".. unknown-thing:: arg\n\n\nAfter.\n", 1},
		{"three leave two", ".. unknown-thing:: arg\n\n\n\nAfter.\n", 2},
		{"at EOF, none", ".. unknown-thing:: arg\n", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			lines := strings.Split(got, "\n")
			i := -1
			for k, l := range lines {
				if strings.Contains(l, "<literal_block>") {
					i = k
					break
				}
			}
			if i < 0 {
				t.Fatalf("no literal_block:\n%s", got)
			}
			if want := ".. unknown-thing:: arg"; !strings.Contains(lines[i+1], want) {
				t.Fatalf("quoted block does not start with the directive:\n%s", got)
			}
			// len(lines)-1 is the empty element Split leaves for the
			// dump's own final newline; it is not a blank line IN the
			// quoted block. Counting it made the EOF case look wrong when
			// the parser was right.
			blanks := 0
			for k := i + 2; k < len(lines)-1 && strings.TrimSpace(lines[k]) == ""; k++ {
				blanks++
			}
			if blanks != tc.wantBlanks {
				t.Errorf("quoted block kept %d blank line(s), want %d:\n%s", blanks, tc.wantBlanks, got)
			}
		})
	}
}
