package rst

import (
	"reflect"
	"regexp"
	"strconv"
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
