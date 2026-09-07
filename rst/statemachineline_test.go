package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestStateMachineLine pins the SECOND line convention this parser has
// to reproduce. Inliner.parse hands its own lineno -- the paragraph's
// FIRST line -- to every diagnostic it raises. A message reported
// WITHOUT one falls back to StateMachine.get_source_and_line() instead,
// and for a top-level paragraph that is max(firstLine+1, lastLine):
// Text.text() steps at least one line past the first looking for a
// continuation, then stops on the last line of the block it read.
//
// Three earlier attempts guessed a formula from too few samples and each
// was contradicted by the next probe ("the line after the paragraph",
// "first + 1", "the duplicate's own line"). What settled it was
// INSTRUMENTING the reference -- base_node.line is always None and the
// node is not yet attached, so the reporter can only be falling back to
// the state machine. Every expectation below is the reference's own.
func TestStateMachineLine(t *testing.T) {
	// The two families that use it, at every paragraph length that
	// distinguishes "first + 1" from "last".
	cases := []struct {
		name, source string
		wantLine     string
	}{
		{
			"duplicate name, one-line paragraph on line 3: first+1 wins",
			"_`dup` here.\n\n`<dup>`_ there.\n", `line="4"`,
		},
		{
			"duplicate name, two-line paragraph on 3-4: the two agree",
			"_`dup` here.\n\nsome text\nonto `<dup>`_ two.\n", `line="4"`,
		},
		{
			"duplicate name, three-line paragraph on 3-5: last wins",
			"_`dup` here.\n\none\ntwo\nand `<dup>`_ three.\n", `line="5"`,
		},
		{
			"duplicate name, one-line paragraph further down",
			"x.\n\ny.\n\n_`dup` here.\n\n`<dup>`_ there.\n", `line="8"`,
		},
		{
			"code role, one-line paragraph on line 4",
			".. role:: tex(code)\n   :language: latex\n\n:tex:`x`.\n", `line="5"`,
		},
		{
			"code role, two-line paragraph on 4-5",
			".. role:: tex(code)\n   :language: latex\n\nCustom role:\n:tex:`x`.\n", `line="5"`,
		},
		{
			"code role, one-line paragraph on line 6",
			".. role:: tex(code)\n   :language: latex\n\ntext.\n\n:tex:`x`.\n", `line="7"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := doctree.Dump(Parse(tc.source)); !strings.Contains(got, tc.wantLine) {
				t.Errorf("missing %s:\n%s", tc.wantLine, got)
			}
		})
	}
}

// TestInlinerLineIsStillTheParagraphsFirst is the control. Inliner's OWN
// diagnostics keep reporting the paragraph's first line, which is what
// makes the state-machine line a second convention rather than a
// correction of the first -- v0.61.0 changed both together and had to be
// reverted whole. A multi-line paragraph is the only shape that can tell
// them apart.
func TestInlinerLineIsStillTheParagraphsFirst(t *testing.T) {
	// Unclosed emphasis on the paragraph's THIRD line; Inliner reports
	// the paragraph's first line (3), not the state machine's (5).
	got := doctree.Dump(Parse("x.\n\none\ntwo\nthree *unclosed\n"))
	if !strings.Contains(got, `line="3"`) {
		t.Errorf("an Inliner diagnostic did not report the paragraph's FIRST line:\n%s", got)
	}
	if strings.Contains(got, `line="5"`) {
		t.Errorf("an Inliner diagnostic picked up the state machine's line:\n%s", got)
	}
}
