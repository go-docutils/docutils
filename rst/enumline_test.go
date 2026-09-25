package rst

import (
	"strconv"
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestEnumeratedListStartValueLine pins the LINE on the "Enumerated list
// start value not ordinal-1" INFO. docutils raises it with
// base_node=enumlist (Body.enumerator, states.py), and
// utils.Reporter.system_message reads base_node's own source/line rather
// than consulting the reporter's cursor at all -- and enumlist.line came
// from state_machine.get_source_and_line(), which a NESTED machine maps
// back through its input_offset to an ABSOLUTE line.
//
// This package passed i+1: the index within whatever slice of lines the
// current nesting level was handed. That is the same number at the top
// level and only there, so every nested list starting at an ordinal other
// than 1 reported its offset within its own block -- one real-world corpus
// file said line="1" for a list on line 704. Seven of the 1564 files, and
// this is the shape of all seven.
//
// The first case is the CONTROL: at the top level the two readings
// coincide, so it passes either way, and it is here to prove the fix did
// not simply shift every line by the wrong amount. The remaining cases
// nest differently -- a bullet item, a block quote, a directive body, two
// bullet levels -- because the offset a nested parse carries is
// established differently in each (get_known_indented vs
// get_first_known_indented vs a directive's own content offset).
//
// Which of them can actually FAIL was checked, not assumed: with the fix
// reverted, the block quote, the directive body and the two-level case all
// fail, and the single bullet item does NOT -- its own index inside the
// item happens to equal the absolute line for this input. It is kept as a
// second control rather than rewritten, since a shape that coincides is
// worth pinning too; the three that discriminate are what guard the rule.
// Every expectation here is the reference's own output for that input.
func TestEnumeratedListStartValueLine(t *testing.T) {
	const msg = "Enumerated list start value not ordinal-1: \"3\" (ordinal 3)"
	cases := []struct {
		name   string
		source string
		want   int
	}{
		{"CONTROL: at the top level, i+1 IS the absolute line", "3. c\n4. d\n", 1},
		{"inside a bullet item, after a paragraph", "* a\n\n  para\n\n  3. c\n  4. d\n", 5},
		{"inside a block quote", "para\n\n   3. c\n   4. d\n", 3},
		{"inside a directive's body", ".. note::\n\n   3. c\n   4. d\n", 3},
		{"two bullet levels down", "* a\n\n  * b\n\n    3. c\n    4. d\n", 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dump := doctree.Dump(Parse(tc.source))
			if !strings.Contains(dump, msg) {
				t.Fatalf("no start-value message at all in:\n%s", dump)
			}
			want := "<system_message level=\"1\" line=\"" + strconv.Itoa(tc.want) + "\" type=\"INFO\">"
			if !strings.Contains(dump, want) {
				t.Errorf("Parse(%q): want %s, dump =\n%s", tc.source, want, dump)
			}
		})
	}
}
