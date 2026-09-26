package rst

import (
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestInlineMessageLineIsTheDocumentMachinesPosition pins the line number
// a message raised during INLINE parsing carries.
//
// docutils does not derive it from the construct at all. Reporter's
// get_source_and_line is bound ONCE, to the DOCUMENT-level state machine
// (RSTStateMachine.run sets it; NestedStateMachine.run does not), and a
// message with no line of its own and no already-attached base_node --
// which is every duplicate-name notice, since the target it describes is
// built before it is parented -- reports wherever THAT machine happens to
// sit. Nothing about the nesting depth enters into it:
//
//   - outside any nested construct (including inside a section, which
//     docutils parses with the document machine itself): max(first+1,
//     last) of the paragraph being parsed;
//   - inside one: FROZEN at the last line of the block the document-level
//     dispatch collected, however deep the message is raised;
//   - and a LIST hands only its first item to that dispatch, so every
//     message anywhere in a list reports the end of item ONE.
//
// This was recorded as "not derivable" for many rounds. It became
// derivable by TRACING the reference -- wrapping get_source_and_line and
// reading back its line_offset and input_offset -- rather than by reading
// states.py again. Every expectation below is that reference's own
// output, for 26 shapes.
func TestInlineMessageLineIsTheDocumentMachinesPosition(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   []int
	}{
		{
			"top 1-line paras",
			"`A <http://e.org/1>`_\n\n`A <http://e.org/2>`_\n",
			[]int{4},
		},
		{
			"top multi-line 2nd",
			"`A <http://e.org/1>`_\n\nfirst line\n`A <http://e.org/2>`_\n",
			[]int{4},
		},
		{
			"top 2nd para 3 lines",
			"`A <http://e.org/1>`_\n\na\nb\n`A <http://e.org/2>`_\n",
			[]int{5},
		},
		{
			"top with trailing para",
			"`A <http://e.org/1>`_\n\n`A <http://e.org/2>`_\n\nafter\n",
			[]int{4},
		},
		{
			"section content",
			"T\n=\n\n`A <http://e.org/1>`_\n\n`A <http://e.org/2>`_\n",
			[]int{7},
		},
		{
			"two sections",
			"T\n=\n\n`A <http://e.org/1>`_\n\nU\n=\n\n`A <http://e.org/2>`_\n",
			[]int{10},
		},
		{
			"quote 1-line",
			"p\n\n   `A <http://e.org/1>`_\n\n   `A <http://e.org/2>`_\n",
			[]int{5},
		},
		{
			"quote multi-line 2nd",
			"p\n\n   `A <http://e.org/1>`_\n\n   a\n   `A <http://e.org/2>`_\n",
			[]int{6},
		},
		{
			"quote then text",
			"p\n\n   `A <http://e.org/1>`_\n\n   `A <http://e.org/2>`_\n\nafter\n",
			[]int{6},
		},
		{
			"quote dup in middle",
			"p\n\n   `A <http://e.org/1>`_\n\n   `A <http://e.org/2>`_\n\n   tail one\n\n   tail two\n",
			[]int{9},
		},
		{
			"quote dup at end, text after",
			"p\n\n   `A <http://e.org/1>`_\n\n   `A <http://e.org/2>`_\n\nafter\n",
			[]int{6},
		},
		{
			"quote deep nest",
			"p\n\n   q\n\n      `A <http://e.org/1>`_\n\n      `A <http://e.org/2>`_\n",
			[]int{7},
		},
		{
			"nested list in quote",
			"p\n\n   * `A <http://e.org/1>`_\n   * `A <http://e.org/2>`_\n",
			[]int{4},
		},
		{
			"quote with list inside",
			"p\n\n   * `A <http://e.org/1>`_\n   * `A <http://e.org/2>`_\n\n   tail\n",
			[]int{6},
		},
		{
			"directive body",
			".. note::\n\n   `A <http://e.org/1>`_\n\n   `A <http://e.org/2>`_\n",
			[]int{5},
		},
		{
			"list 1-line items",
			"* `A <http://e.org/1>`_\n* `A <http://e.org/2>`_\n",
			[]int{1},
		},
		{
			"list 2nd item 2 lines",
			"* `A <http://e.org/1>`_\n* a\n  `A <http://e.org/2>`_\n",
			[]int{1},
		},
		{
			"list 2nd item para 2",
			"* `A <http://e.org/1>`_\n\n* a\n\n  `A <http://e.org/2>`_\n",
			[]int{2},
		},
		{
			"list then text",
			"* `A <http://e.org/1>`_\n* `A <http://e.org/2>`_\n\nafter\n",
			[]int{1},
		},
		{
			"list more items after",
			"* `A <http://e.org/1>`_\n* `A <http://e.org/2>`_\n* third\n* fourth\n",
			[]int{1},
		},
		{
			"list long first item",
			"* a\n\n  b\n\n  `A <http://e.org/1>`_\n\n* `A <http://e.org/2>`_\n",
			[]int{6},
		},
		{
			"field list",
			":f: `A <http://e.org/1>`_\n:g: `A <http://e.org/2>`_\n",
			[]int{1},
		},
		{
			"field body 2 lines",
			":f: `A <http://e.org/1>`_\n:g: a\n    `A <http://e.org/2>`_\n",
			[]int{1},
		},
		{
			"definition list",
			"t1\n   `A <http://e.org/1>`_\nt2\n   `A <http://e.org/2>`_\n",
			[]int{2},
		},
		{
			"table cell",
			"+----+\n| `A <http://e.org/1>`_ |\n+----+\n| `A <http://e.org/2>`_ |\n+----+\n",
			[]int{2},
		},
		{
			"line block",
			"| `A <http://e.org/1>`_\n| `A <http://e.org/2>`_\n",
			nil,
		},
	}
	re := regexp.MustCompile(`<system_message[^>]*line="(\d+)"`)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []int
			for _, m := range re.FindAllStringSubmatch(doctree.Dump(Parse(tc.source)), -1) {
				n, _ := strconv.Atoi(m[1])
				got = append(got, n)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse(%q): message lines = %v, want %v\n%s", tc.source, got, tc.want, doctree.Dump(Parse(tc.source)))
			}
		})
	}
}

// TestContentExpectedLineIsTheCursorTrailingBlanksIncluded is the other
// half of the same rule. docutils raises this one as
// reporter.warning('Citation content expected.') with NO line argument at
// all, so system_message falls back to the document machine's own
// position -- and that machine stops on the last line it CONSUMED, which
// includes the blank line(s) that terminated the enclosing block
// (get_indented reads them and leaves the cursor on the last one).
//
// This package computed the line from the construct's own block instead.
// That is the same number at the top level and only there: inside a
// directive or footnote body the block has no trailing blank (the
// gatherer trims it), so every such message came out one line early --
// two lines early where two blank lines followed.
//
// Found by a 216-case differential probe (18 diagnostic-producing
// snippets by 12 nesting wrappers, /Users/Shared/rstcorpus/nestprobe):
// five shapes differed, all of them this one, and neither corpus contains
// any of them -- 578/579 and 1553/1564 are identical before and after. The
// three top-level cases are the CONTROLS: they were already right, and a
// rule that "fixed" them would have broken what the corpus does cover.
func TestContentExpectedLineIsTheCursorTrailingBlanksIncluded(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   int
	}{
		{"inside a directive, one blank line after it", "intro\n\n.. note::\n\n   .. [c]\n\nlast\n", 6},
		{"inside a directive, TWO blank lines after it", "intro\n\n.. note::\n\n   .. [c]\n\n\nlast\n", 7},
		{"inside a directive with more content after the citation", "intro\n\n.. note::\n\n   .. [c]\n\n   after\n\nlast\n", 8},
		{"inside a directive, two citations", "intro\n\n.. note::\n\n   .. [c]\n   .. [d]\n\nlast\n", 7},
		{"inside a footnote body", "intro\n\n.. [9]\n   .. [c]\n\nlast\n", 5},
		{"a directive inside a bullet item", "intro\n\n- item\n\n  .. note::\n\n     .. [c]\n\nlast\n", 8},
		{"CONTROL: at EOF there is no blank line to consume", "intro\n\n.. note::\n\n   .. [c]\n", 5},
		{"CONTROL: top level, one blank line", "intro\n\n.. [c]\n\nlast\n", 4},
		{"CONTROL: top level, two blank lines", "intro\n\n.. [c]\n\n\nlast\n", 5},
		{"CONTROL: top level at EOF", "intro\n\n.. [c]\n", 3},
	}
	re := regexp.MustCompile(`<system_message level="2" line="(\d+)" type="WARNING">\n\s*<paragraph>\n\s*Citation content expected\.`)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dump := doctree.Dump(Parse(tc.source))
			m := re.FindStringSubmatch(dump)
			if m == nil {
				t.Fatalf("no citation-content warning with a line:\n%s", dump)
			}
			if got, _ := strconv.Atoi(m[1]); got != tc.want {
				t.Errorf("line = %d, want %d:\n%s", got, tc.want, dump)
			}
		})
	}
}

// TestNestedTitleAttempt pins Text.underline's own order of checks in a
// NESTED (match_titles=False) context, where a section title cannot
// appear: the width rule runs FIRST, then the match_titles one.
//
//   - an underline has NO minimum length (docutils' underline pattern is
//     the `line` pattern, one punctuation character repeated), so "dup"
//     over "===" is a title attempt and draws the ERROR. This package
//     tested isTransitionLine, whose four-character floor belongs to
//     TRANSITIONS, and produced a paragraph instead;
//   - a title WIDER than its underline is not a title at all when the
//     underline is under four characters (TransitionCorrection sends both
//     lines back as text), and merely too short above it -- where the
//     WARNING is emitted BEFORE the error, which this path omitted;
//   - a DEMOTED short adornment ("..." over "===") comes back through the
//     dispatch as a title's own text, so it draws the "so short" INFO and
//     then the ERROR, where this package swallowed both lines into one
//     paragraph.
//
// Found by the 348-case nesting probe
// (/Users/Shared/rstcorpus/nestprobe): 22 shapes differed across 11
// wrappers, none of them present in either corpus, which stays at 578/579
// and 1553/1564 before and after. The top-level rows are the CONTROLS:
// there the same inputs build real sections, and a rule that reported
// these unconditionally would have broken every short-underline title the
// corpus does cover.
func TestNestedTitleAttempt(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   []string
	}{
		{"three-character underline under a three-character title", "intro\n\n   dup\n   ===\n\nlast\n",
			[]string{`<system_message level="3" line="4" type="ERROR">`, "Unexpected section title."}},
		{"a one-character underline is still an underline", "intro\n\n   a\n   =\n\nlast\n",
			[]string{`line="4"`, "Unexpected section title."}},
		{"too short for its title, but four characters: WARNING then ERROR", "intro\n\n   Title\n   ====\n\nlast\n",
			[]string{"Title underline too short.", "Unexpected section title."}},
		{"a demoted short adornment comes back as a title's text", "intro\n\n   ...\n   ===\n\nlast\n",
			[]string{"Treating it as ordinary text because it's so short.", "Unexpected section title."}},
		{"CONTROL: a title wider than an under-4 underline is plain text", "intro\n\n   Title\n   ===\n\nlast\n",
			[]string{"<paragraph>"}},
		{"CONTROL: at the top level the same pair is a section", "intro\n\ndup\n===\n\nlast\n",
			[]string{"<section", "<title>"}},
		{"CONTROL: at the top level a one-character underline is a section too", "intro\n\na\n=\n\nlast\n",
			[]string{"<section"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dump := doctree.Dump(Parse(tc.source))
			for _, want := range tc.want {
				if !strings.Contains(dump, want) {
					t.Errorf("missing %q:\n%s", want, dump)
				}
			}
			if tc.name == "CONTROL: a title wider than an under-4 underline is plain text" &&
				strings.Contains(dump, "Unexpected section title.") {
				t.Errorf("reported a title where the reference has plain text:\n%s", dump)
			}
		})
	}
}
