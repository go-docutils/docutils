package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestShallowContinuationDiffersByConstruct pins a rule that is NOT
// uniform across constructs, which is exactly why one shared helper got it
// wrong for half of them.
//
// A FIELD BODY and an OPTION DESCRIPTION discover their own indent
// (docutils' get_first_known_indented), so a continuation indented LESS
// than the marker column still belongs to them. A LIST ITEM does not: its
// content column is fixed by the marker, and a shallower line ENDS the
// list, becoming a block quote with an "ends without a blank line"
// warning. All four were checked against the reference.
func TestShallowContinuationDiffersByConstruct(t *testing.T) {
	joins := []struct{ name, source, wantText string }{
		{"a field body absorbs a shallower continuation", ":date: 2026-08-30\n  continuation\n", "continuation"},
		{"an option description absorbs one too", "-a  option one\n  continuation\n", "continuation"},
	}
	for _, tc := range joins {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if strings.Contains(got, "block_quote") {
				t.Errorf("the continuation was split off instead of joining:\n%s", got)
			}
			if !strings.Contains(got, tc.wantText) {
				t.Errorf("the continuation text vanished:\n%s", got)
			}
		})
	}

	ends := []struct{ name, source, wantWarning string }{
		{"an enumerated list ENDS at a shallower continuation", "1. Item one\n  continuation\n",
			"Enumerated list ends without a blank line; unexpected unindent."},
		{"a bullet list ends the same way", "- Item one\n continuation\n",
			"Bullet list ends without a blank line; unexpected unindent."},
	}
	for _, tc := range ends {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !strings.Contains(got, tc.wantWarning) {
				t.Errorf("expected %q:\n%s", tc.wantWarning, got)
			}
			if !strings.Contains(got, "<block_quote>") {
				t.Errorf("the shallower line should have become a block quote:\n%s", got)
			}
		})
	}
}

// TestBareMarkerTakesContentColumnFromItsFirstLine covers the half of the
// rule that is INDEPENDENT of the construct: a marker with nothing after
// it has no anchor, so the content column is wherever the first indented
// line actually starts -- narrower or wider than the marker alike. A first
// version of the guard above dropped the narrower half and emptied every
// one of these items.
func TestBareMarkerTakesContentColumnFromItsFirstLine(t *testing.T) {
	for _, src := range []string{
		"1.\n   foo\n", // exactly the marker width
		"1.\n  foo\n",  // narrower
		"1.\n foo\n",   // narrower still
	} {
		got := doctree.Dump(Parse(src))
		if !strings.Contains(got, "foo") {
			t.Errorf("Parse(%q) lost the item's content:\n%s", src, got)
		}
		if strings.Contains(got, "<block_quote>") {
			t.Errorf("Parse(%q) split the content into a block quote:\n%s", src, got)
		}
	}
}
