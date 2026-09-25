package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestIndentWidthCountsUnicodeWhitespace covers an asymmetry docutils has
// and this package had collapsed into one rule. "Is this line indented?"
// is `line[0] != ' '` — a PLAIN space. "How deep is it?" is
// `len(line) - len(line.lstrip())`, and str.lstrip() strips every Unicode
// whitespace character, a NO-BREAK SPACE included.
//
// pytest's own documentation has a literal block whose first line is
// written "     pytest …": one plain space by the first test,
// five CHARACTERS deep by the second, and docutils dedents all five.
// Counting plain spaces dedented one, so the block kept four characters
// of phantom indentation — visible in the rendered page, since a literal
// block preserves it.
//
// The last two cases are the CONTROLS, and they are why the two questions
// cannot be merged: a line indented with NO-BREAK SPACES ALONE is not
// indented at all. The block quote becomes an ordinary paragraph and the
// literal block is "expected; none found". Every expectation is real
// docutils 0.23's own output.
func TestIndentWidthCountsUnicodeWhitespace(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a literal block whose indent mixes spaces and no-break spaces",
			"Run it::\n\n     cmd one\n     cmd two\n",
			"<literal_block>\n        cmd one\n        cmd two",
		},
		{
			"a block quote indented the same way",
			"Para.\n\n     quoted one\n     quoted two\n",
			"<block_quote>\n        <paragraph>\n            quoted one\n            quoted two",
		},
		{
			// CONTROL: no plain space at all, so nothing is indented.
			"no-break spaces alone do not indent a block quote",
			"Para.\n\n    quoted\n",
			"<paragraph>\n        Para.\n    <paragraph>\n            quoted",
		},
		{
			// CONTROL, and the sharper one: the literal block the "::"
			// promised is MISSING, with the warning docutils raises.
			"no-break spaces alone leave a literal block unfound",
			"Run it::\n\n    cmd\n",
			"Literal block expected; none found.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !strings.Contains(got, tc.want) {
				t.Errorf("Parse(%q) dump =\n%s\nwant it to contain:\n%s", tc.source, got, tc.want)
			}
		})
	}
}

// TestUnknownRoleMessagesQuoteTheRoleAsWritten covers the three messages
// an unknown role raises. roles.role builds its two from "role_name" and
// Inliner.interpreted its one from `role` — all three the name AS THE
// AUTHOR WROTE IT. The lowercased form exists for LOOKUP only, and this
// package quoted that instead, so ":Class:`x`" reported "class".
//
// The two directive paths that raise the same pair (".. role::" with an
// unknown base, ".. default-role::" with an unknown name) already had it
// right, which is the control: one file and one capital letter is all the
// corpus needed to show that two of three call sites agreed and the third
// did not.
func TestUnknownRoleMessagesQuoteTheRoleAsWritten(t *testing.T) {
	cases := []struct {
		name, source, written string
	}{
		{"an inline role", "See :Class:`x` here.\n", "Class"},
		{"mixed case", "See :MiXeD:`x` here.\n", "MiXeD"},
		{"already lowercase", "See :nosuch:`x` here.\n", "nosuch"},
		{"the role directive's base", ".. role:: mine(BogusBase)\n", "BogusBase"},
		{"the default-role directive", ".. default-role:: BogusName\n", "BogusName"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			for _, want := range []string{
				`No role entry for "` + tc.written + `" in module`,
				`Trying "` + tc.written + `" as canonical role name.`,
				`Unknown interpreted text role "` + tc.written + `".`,
			} {
				if !strings.Contains(got, want) {
					t.Errorf("missing %q in:\n%s", want, got)
				}
			}
		})
	}
}
