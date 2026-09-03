package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

func TestMatchFieldMarker(t *testing.T) {
	cases := []struct {
		line     string
		wantName string
		wantCol  int
		wantOK   bool
	}{
		{":author: Jane Doe", "author", 9, true},
		{":date:", "date", 6, true},
		{"", "", 0, false},
		{":", "", 0, false},
		{":: not a field", "", 0, false},
		{": not a field either", "", 0, false},
		{"not a field at all", "", 0, false},
		{":unterminated field name", "", 0, false},
		// docutils/rst v0.39.0+ — an escaped colon ("\:", consumed as ONE
		// atomic unit, "\\." in the real pattern) is always name content,
		// never a candidate close.
		{`:Field\: names\: with\: colons\:: are possible.`, `Field\: names\: with\: colons\:`, 34, true},
		{`:\: Not a field list either.`, "", 0, false},
		// A space right before the closing colon disqualifies it — and
		// since nothing else in the pattern can consume a colon that's
		// ALSO followed by a space, the whole match fails outright rather
		// than continuing to hunt for a later colon.
		{":Field : marker must not end with whitespace.", "", 0, false},
	}
	for _, tc := range cases {
		name, col, ok := matchFieldMarker(tc.line)
		if ok != tc.wantOK || name != tc.wantName || (ok && col != tc.wantCol) {
			t.Errorf("matchFieldMarker(%q) = (%q, %d, %v), want (%q, %d, %v)",
				tc.line, name, col, ok, tc.wantName, tc.wantCol, tc.wantOK)
		}
	}
}

func TestIsDefinitionTermLine(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		i     int
		want  bool
	}{
		{"term followed by indented line", []string{"Term", "    body"}, 0, true},
		{"blank line breaks term detection", []string{"Term", ""}, 0, false},
		{"next line not indented", []string{"Term", "Not indented"}, 0, false},
		{"last line has no successor", []string{"Term"}, 0, false},
		{"blank line itself is never a term", []string{"", "    body"}, 0, false},
		{"indented line is never a term", []string{"  indented", "    body"}, 0, false},
		{"bullet line is never a term", []string{"- item", "    body"}, 0, false},
		{"enum line is never a term", []string{"1. item", "    body"}, 0, false},
		{"explicit markup line is never a term", []string{".. comment", "    body"}, 0, false},
		{"field marker line is never a term", []string{":field: x", "    body"}, 0, false},
		{"uniform-punctuation line is never a term", []string{"====", "    body"}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDefinitionTermLine(tc.lines, tc.i); got != tc.want {
				t.Errorf("isDefinitionTermLine(%v, %d) = %v, want %v", tc.lines, tc.i, got, tc.want)
			}
		})
	}
}

// TestFieldListDiagnostics covers docutils' Body.field/field_marker
// (states.py, read directly) diagnostics ported this round: "Field list
// ends without a blank line; unexpected unindent." (the same shared
// unindent_warning shape footnotes/citations, line blocks, and
// definition lists already have), and a field name's own real line
// number for its inline-markup diagnostics. A field list that would be
// promoted to <docinfo> (the document's very first child) is
// deliberately avoided in every case here — see promoteDocInfo's own
// doc comment and this session's memory: real docutils' DocInfo
// promotion is itself a TRANSFORM, never applied to the testsuite
// corpus's own bare-parse ground truth, so this project's eager
// promotion (kept because go-richdoc/rst's Document.Meta genuinely
// needs it) is a real, deliberate, already-precedented divergence from
// the corpus for a LEADING field list specifically — not something
// these cases are trying to exercise.
func TestFieldListDiagnostics(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a field list interrupted by a non-blank line warns about the missing blank line",
			"Some edge cases:\n\n:Empty:\n:Author: Me\nNo blank line before this paragraph.\n",
			"<document>\n    <paragraph>\n        Some edge cases:\n    <field_list>\n        <field>\n            <field_name>\n                Empty\n            <field_body>\n        <field>\n            <field_name>\n                Author\n            <field_body>\n                <paragraph>\n                    Me\n    <system_message level=\"2\" line=\"5\" type=\"WARNING\">\n        <paragraph>\n            Field list ends without a blank line; unexpected unindent.\n    <paragraph>\n        No blank line before this paragraph.\n",
		},
		{
			"a field name's own inline-markup diagnostic carries a real line number",
			"Some text.\n\n:Field name with *bad inline markup: should generate warning.\n",
			"<document>\n    <paragraph>\n        Some text.\n    <field_list>\n        <field>\n            <field_name>\n                Field name with \n                <problematic id=\"problematic-1\" refid=\"system-message-1\">\n                    *\n                bad inline markup\n            <field_body>\n                <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"2\" line=\"3\" type=\"WARNING\">\n                    <paragraph>\n                        Inline emphasis start-string without end-string.\n                <paragraph>\n                    should generate warning.\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if strings.TrimRight(got, "\n") != strings.TrimRight(tc.want, "\n") {
				t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
}
