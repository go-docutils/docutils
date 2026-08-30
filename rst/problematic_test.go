package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestDanglingReferenceBecomesProblematic covers docutils' own
// DanglingReferences + Messages transforms, simplified: a NAMED reference
// (bare, backtick-quoted, or an embedded indirect alias — an ANONYMOUS
// reference is checked differently, by whole-document count mismatch, see
// TestAnonymousMismatchBecomesProblematic below) with no matching target
// anywhere is rewritten to a <problematic> in place, and every such
// message collects into one trailing <section class="system-messages">
// at the very end of the document, docutils' own "loose messages get a
// dedicated section" convention.
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

// TestAnonymousMismatchBecomesProblematic covers docutils'
// AnonymousHyperlinks.apply, read directly from transforms/references.py:
// unlike a named reference, an anonymous reference/target mismatch is a
// single whole-document condition (count `!=`, checked once — an earlier
// pass at this got the direction wrong and treated it as "too many
// references", which the second case below specifically guards against),
// and when it doesn't match EVERY anonymous reference in the document
// becomes <problematic>, all sharing ONE message.
func TestAnonymousMismatchBecomesProblematic(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"more references than targets",
			"Too many anon refs: first__ second__.\n",
			"<document>\n    <paragraph>\n        Too many anon refs: \n        <problematic id=\"problematic-2\" refid=\"system-message-1\">\n            first\n         \n        <problematic id=\"problematic-3\" refid=\"system-message-1\">\n            second\n        .\n    <section class=\"system-messages\">\n        <title>\n            Docutils System Messages\n        <system_message backref=\"problematic-2 problematic-3\" id=\"system-message-1\">\n            <paragraph>\n                Anonymous hyperlink mismatch: 2 references but 0 targets.\n",
		},
		{
			"MORE TARGETS than references also mismatches — not just an excess of references",
			".. __: https://a.example\n.. __: https://b.example\n\nOnly one used: first__.\n",
			"<document>\n    <target anonymous=\"true\" refuri=\"https://a.example\">\n    <target anonymous=\"true\" refuri=\"https://b.example\">\n    <paragraph>\n        Only one used: \n        <problematic id=\"problematic-2\" refid=\"system-message-1\">\n            first\n        .\n    <section class=\"system-messages\">\n        <title>\n            Docutils System Messages\n        <system_message backref=\"problematic-2\" id=\"system-message-1\">\n            <paragraph>\n                Anonymous hyperlink mismatch: 1 references but 2 targets.\n",
		},
		{
			"a balanced count is unaffected, no trailing section at all",
			".. __: https://a.example\n.. __: https://b.example\n\nBoth used: first__ and second__.\n",
			"<document>\n    <target anonymous=\"true\" refuri=\"https://a.example\">\n    <target anonymous=\"true\" refuri=\"https://b.example\">\n    <paragraph>\n        Both used: \n        <reference anonymous=\"true\" name=\"first\" refuri=\"https://a.example\">\n            first\n         and \n        <reference anonymous=\"true\" name=\"second\" refuri=\"https://b.example\">\n            second\n        .\n",
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
