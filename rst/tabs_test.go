package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestExpandTabs pins the arithmetic: a tab advances to the next
// multiple of 8, counting COLUMNS, so its width depends on where it
// sits. Python's str.expandtabs, which string2lines applies to every
// input line.
func TestExpandTabs(t *testing.T) {
	cases := map[string]string{
		"\ta":         "        a",
		"a\tb":        "a       b",
		"ab\tc":       "ab      c",
		"abcdefg\th":  "abcdefg h",
		"abcdefgh\ti": "abcdefgh        i",
		"   \tx":      "        x",
		"a\t\tb":      "a               b",
		"no tabs":     "no tabs",
	}
	for in, want := range cases {
		if got := expandTabs(in, 8); got != want {
			t.Errorf("expandTabs(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestTabIndentedBlocks covers what the expansion is FOR. docutils runs
// string2lines over its input, so every line arrives with tabs expanded
// and trailing whitespace stripped. Without that a leading tab counted
// as ZERO indent, and a tab-indented block escaped whatever construct
// it belonged to -- sphinx's C-domain documentation indents with tabs,
// so 14 real-world files turned on it.
//
// Every expectation is the reference's own.
func TestTabIndentedBlocks(t *testing.T) {
	cases := []struct {
		name, source string
		want         []string
	}{
		{
			"a tab-indented block is a block quote",
			"text\n\n\tquoted\n",
			[]string{"<block_quote>"},
		},
		{
			"a tab inside a line becomes spaces to the next stop",
			"a\tb\n",
			[]string{"a       b"},
		},
		{
			// The shape that found this: tab-indented content under a
			// directive belongs to the directive.
			"a tab-indented bullet list under a directive",
			".. note::\n\n\t- one\n\t- two\n",
			[]string{"<note>", `<bullet_list bullet="-">`},
		},
		{
			// A tab after three spaces advances to column 8, not by 8.
			"a tab after spaces still lands on the stop",
			"text\n\n   \tdeep\n",
			[]string{"<block_quote>"},
		},
		{
			// VT and FF are converted to SPACES before the split, so
			// neither breaks a line -- "a\vb" is one paragraph reading
			// "a b". U+2028 IS a boundary, which is the control that
			// keeps this from being "no exotic character splits".
			"a vertical tab is a space, not a line break",
			"a\vb\n",
			[]string{"<paragraph>\n        a b\n"},
		},
		{
			"a form feed likewise",
			"a\fb\n",
			[]string{"<paragraph>\n        a b\n"},
		},
		{
			"but U+2028 really is a line break",
			"a\u2028b\n",
			[]string{"<paragraph>\n        a\n        b\n"},
		},
		{
			"trailing whitespace is stripped from every line",
			"a   \n\nb\t\n",
			[]string{"<paragraph>\n        a\n", "<paragraph>\n        b\n"},
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
		})
	}
}
