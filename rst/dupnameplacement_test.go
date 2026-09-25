package rst

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

const (
	dupFirst  = "`A <http://e.org/1>`_"
	dupSecond = "`A <http://e.org/2>`_"
)

// containerOf returns the tag of the nearest ancestor holding the first
// <system_message> in a dump — read off the dump's own indentation, which
// is what "where did the message land" means here.
func containerOf(dump string) string {
	lines := strings.Split(dump, "\n")
	for i, l := range lines {
		if !strings.Contains(l, "<system_message") {
			continue
		}
		indent := len(l) - len(strings.TrimLeft(l, " "))
		for j := i - 1; j >= 0; j-- {
			p := lines[j]
			if len(p)-len(strings.TrimLeft(p, " ")) < indent && strings.HasPrefix(strings.TrimSpace(p), "<") {
				return strings.Trim(strings.Fields(strings.TrimSpace(p))[0], "<>")
			}
		}
	}
	return "NO MESSAGE"
}

// TestDuplicateNameMessageLandsInItsContainer covers WHERE a duplicate-name
// notice goes. docutils appends it to whatever body element is being
// filled at that moment, so it lands inside the innermost container,
// immediately before the paragraph whose inline parsing raised it.
//
// The list of tags that can hold one named ".. admonition::" and not one
// of the NINE specific admonitions it shares its content model with — so
// the notice for a duplicate inside a ".. note::" went BEFORE the note.
// PEP 813 is the corpus file; every container below was then checked
// against the reference one at a time, because a FAMILY is exactly what a
// list like that gets wrong.
func TestDuplicateNameMessageLandsInItsContainer(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"note", ".. note::\n\n   %s\n", "note"},
		{"hint", ".. hint::\n\n   %s\n", "hint"},
		{"warning", ".. warning::\n\n   %s\n", "warning"},
		{"caution", ".. caution::\n\n   %s\n", "caution"},
		{"danger", ".. danger::\n\n   %s\n", "danger"},
		{"error", ".. error::\n\n   %s\n", "error"},
		{"important", ".. important::\n\n   %s\n", "important"},
		{"tip", ".. tip::\n\n   %s\n", "tip"},
		{"attention", ".. attention::\n\n   %s\n", "attention"},
		// header and footer admit body elements too, and the reference
		// does put the notice inside them -- but no case is listed here,
		// because this package does not RAISE one there at all: the
		// duplicate-name pass runs before hoistDecoration, so a header's
		// own subtree is not in the document yet when names are
		// registered. Moving the hoist earlier is not enough either, since
		// the walk would then see the header BEFORE the line-1 reference
		// it follows in the source, and docutils registers names in SOURCE
		// order. No corpus file has a duplicate name inside a header, so
		// this is written down rather than chased.
		// CONTROLS: the containers that were already right. They share
		// the same rule, so a change that broke it would show here too.
		{"generic admonition", ".. admonition:: T\n\n   %s\n", "admonition"},
		{"topic", ".. topic:: T\n\n   %s\n", "topic"},
		{"sidebar", ".. sidebar:: T\n\n   %s\n", "sidebar"},
		{"container", ".. container:: c\n\n   %s\n", "container"},
		{"compound", ".. compound::\n\n   %s\n", "compound"},
		{"list item", "* %s\n", "list_item"},
		{"block quote", "   %s\n", "block_quote"},
		{"field body", ":f: %s\n", "field_body"},
		{"definition", "term\n   %s\n", "definition"},
		// CONTROL: at top level the container IS the document.
		{"top level", "%s\n", "document"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := dupFirst + "\n\n" + strings.Replace(tc.source, "%s", dupSecond, 1)
			got := containerOf(doctree.Dump(Parse(src)))
			if got != tc.want {
				t.Errorf("Parse(%q): the message landed in <%s>, want <%s>\n%s",
					src, got, tc.want, doctree.Dump(Parse(src)))
			}
		})
	}
}

// TestInlineMessageLineInsideADirective pins the line such a notice
// carries when it is raised inside a directive: the document machine
// stopped at the end of the DIRECTIVE's own block, and
// get_first_known_indented collects the trailing blank lines too — so the
// number GROWS with them, which is the shape that says it is the block's
// end and not the content's.
func TestInlineMessageLineInsideADirective(t *testing.T) {
	re := regexp.MustCompile(`<system_message[^>]*line="(\d+)"`)
	cases := []struct {
		name   string
		source string
		want   int
	}{
		{"one blank line after the note", ".. note::\n\n   %s\n\nAfter.\n", 6},
		{"two blank lines after", ".. note::\n\n   %s\n\n\nAfter.\n", 7},
		{"three blank lines after", ".. note::\n\n   %s\n\n\n\nAfter.\n", 8},
		{"the note ends the document", ".. note::\n\n   %s\n", 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := dupFirst + "\n\n" + strings.Replace(tc.source, "%s", dupSecond, 1)
			m := re.FindStringSubmatch(doctree.Dump(Parse(src)))
			if m == nil {
				t.Fatalf("Parse(%q) raised no message:\n%s", src, doctree.Dump(Parse(src)))
			}
			got, _ := strconv.Atoi(m[1])
			if got != tc.want {
				t.Errorf("Parse(%q): message line = %d, want %d", src, got, tc.want)
			}
		})
	}
}
