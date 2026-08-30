package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestDanglingReferenceBecomesProblematic covers docutils' own
// DanglingReferences + Messages transforms, simplified: a NAMED reference
// (bare, backtick-quoted, or an embedded indirect alias — anonymous
// references are not covered, see linkReferences' own doc comment) with no
// matching target anywhere is rewritten to a <problematic> in place, and
// every such message collects into one trailing
// <section class="system-messages"> at the very end of the document,
// docutils' own "loose messages get a dedicated section" convention.
func TestDanglingReferenceBecomesProblematic(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a single dangling bare reference",
			"See broken_ reference.\n",
			"<document>\n    <paragraph>\n        See \n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            broken\n         reference.\n    <section class=\"system-messages\">\n        <title>\n            Docutils System Messages\n        <system_message backref=\"problematic-1\" id=\"system-message-1\">\n            <paragraph>\n                Unknown target name: \"broken\".\n",
		},
		{
			"two dangling references collect two separate, correctly cross-linked messages",
			"Two dangling: first_ and second_.\n",
			"<document>\n    <paragraph>\n        Two dangling: \n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            first\n         and \n        <problematic id=\"problematic-2\" refid=\"system-message-2\">\n            second\n        .\n    <section class=\"system-messages\">\n        <title>\n            Docutils System Messages\n        <system_message backref=\"problematic-1\" id=\"system-message-1\">\n            <paragraph>\n                Unknown target name: \"first\".\n        <system_message backref=\"problematic-2\" id=\"system-message-2\">\n            <paragraph>\n                Unknown target name: \"second\".\n",
		},
		{
			"a resolved reference is unaffected, no trailing section at all",
			"A real one: `Python <https://python.org>`_.\n",
			"<document>\n    <paragraph>\n        A real one: \n        <reference name=\"Python\" refuri=\"https://python.org\">\n            Python\n        .\n",
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

// TestAnonymousReferenceMismatchStaysUnresolved guards the deliberate
// scope boundary linkReferences' own doc comment describes: an anonymous
// reference with no available anonymous target left to consume is NOT
// rewritten to <problematic> — real docutils reports a different error
// for this (a position/count mismatch, not a missing name), which this
// project's existing simplification already didn't replicate before this
// feature, and still doesn't now.
func TestAnonymousReferenceMismatchStaysUnresolved(t *testing.T) {
	got := doctree.Dump(Parse("Too many anon refs: first__ second__.\n"))
	if strings.Contains(got, "problematic") {
		t.Errorf("an unmatched anonymous reference was rewritten to problematic, want it left alone:\n%s", got)
	}
}
