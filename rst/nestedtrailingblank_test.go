package rst

import (
	"reflect"
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestNestedBlockKeepsTrailingBlanks covers the quoted block a
// diagnostic carries when the construct it describes sits INSIDE
// something else.
//
// docutils' StringList.get_indented stops at the first insufficiently
// indented NON-BLANK line and trims nothing from the end, so the blank
// lines after a directive belong to the block that quotes it — two
// written, one showing in the dump (the last one is the text's own
// trailing newline). This package's three nested-block gatherers each
// stripped every trailing blank, so a nested directive's quoted block
// showed none at all: three real-world files, in a list item, a field
// body and a block quote.
//
// The experiment that established this cost three testsuite cases and a
// real-world file the first time it was tried, and the whole of that was
// ONE emptiness test: an option marker with nothing after it now yields
// one blank line rather than an empty slice, and the option list read
// that as a description. The control below is that marker.
func TestNestedBlockKeepsTrailingBlanks(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   []string
	}{
		{
			"inside a list item",
			"- item\n\n  .. unknown:: 5.1\n\n\nTitle\n=====\n\nx\n",
			[]string{".. unknown:: 5.1", ""},
		},
		{
			"inside a field body",
			":f: value\n\n    .. unknown:: x\n\n\nTitle\n=====\n\ny\n",
			[]string{".. unknown:: x", ""},
		},
		{
			"inside a block quote",
			"Para.\n\n   quoted\n\n   .. unknown:: x\n\n\nTitle\n=====\n\ny\n",
			[]string{".. unknown:: x", ""},
		},
		{
			// CONTROL: ONE blank line leaves nothing behind. The rule is
			// "one fewer than were written", not "always one".
			"one blank line leaves no trailing blank",
			"- item\n\n  .. unknown:: 5.1\n\nTitle\n=====\n\nx\n",
			[]string{".. unknown:: 5.1"},
		},
		{
			// CONTROL: and THREE leave two, so this is a count, not a
			// flag.
			"three blank lines leave two",
			"- item\n\n  .. unknown:: 5.1\n\n\n\nTitle\n=====\n\nx\n",
			[]string{".. unknown:: 5.1", "", ""},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := literalBlockLines(doctree.Dump(Parse(tc.source)))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse(%q): quoted block = %q, want %q", tc.source, got, tc.want)
			}
		})
	}
}

// literalBlockLines returns the first <literal_block>'s own lines from a
// dump, dedented — the dump indents them by their depth in the tree, and
// this test is about what the block CONTAINS, not about where it sits.
func literalBlockLines(dump string) []string {
	lines := strings.Split(dump, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "<literal_block>" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " ")) + 4
		var out []string
		for _, l := range lines[i+1:] {
			if strings.TrimSpace(l) != "" && len(l)-len(strings.TrimLeft(l, " ")) < indent {
				break
			}
			if len(l) >= indent {
				out = append(out, strings.TrimRight(l[indent:], " "))
			} else {
				out = append(out, "")
			}
		}
		return out
	}
	return nil
}

// TestOptionMarkerWithNoDescription is the control that the emptiness
// test above talks about, kept as its own test because it is the ONE
// thing that broke when the gatherers stopped trimming: "-f" alone is a
// paragraph, not an option list with an empty description.
func TestOptionMarkerWithNoDescription(t *testing.T) {
	cases := []struct{ source, want string }{
		{"-f\n", "<document>\n    <paragraph>\n        -f\n"},
		{"-f\n\n", "<document>\n    <paragraph>\n        -f\n"},
		{"-f  description\n", "<document>\n    <option_list>\n        <option_list_item>\n            <option_group>\n                <option>\n                    <option_string>\n                        -f\n            <description>\n                <paragraph>\n                    description\n"},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if strings.TrimRight(got, "\n") != strings.TrimRight(tc.want, "\n") {
				t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
}
