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

// TestUnclosedInlineMarkupBecomesProblematic covers real docutils'
// Inliner.inline_obj — a SEPARATE <problematic> source from the two above,
// fired during inline parsing itself (inline.go's tryMarker/
// tryInterpretedOrPhraseRef), not a whole-document post-pass: an inline
// markup start-string (single or double asterisk, two backticks, or a
// backquote) with no matching end-string becomes a <problematic> wrapping
// just the marker text, cross
// linked to a message the same way, and — checked against the foreign
// judge, not assumed — substitution_reference ("|") never actually warns
// this way despite routing through the identical inline_obj machinery in
// real docutils; a role-prefixed backquote (":role:`unclosed) still ends
// up byte-identical to real docutils even though this parser never builds
// it in one step, since parseInline's own per-rune fallback naturally
// retries at the backquote's own position with no role prefix in front of
// it by the time it gets there.
func TestUnclosedInlineMarkupBecomesProblematic(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a resolved emphasis is unaffected, no trailing section at all",
			"a resolved *emphasis* is unaffected, no trailing section at all.\n",
			"<document>\n    <paragraph>\n        a resolved \n        <emphasis>\n            emphasis\n         is unaffected, no trailing section at all.\n",
		},
		{
			"two independent unclosed markers collect two separate, correctly numbered messages",
			"Two unclosed: *first and **second here.\n",
			"<document>\n    <paragraph>\n        Two unclosed: \n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            *\n        first and \n        <problematic id=\"problematic-2\" refid=\"system-message-2\">\n            **\n        second here.\n    <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"2\" line=\"1\" type=\"WARNING\">\n        <paragraph>\n            Inline emphasis start-string without end-string.\n    <system_message backref=\"problematic-2\" id=\"system-message-2\" level=\"2\" line=\"1\" type=\"WARNING\">\n        <paragraph>\n            Inline strong start-string without end-string.\n",
		},
		{
			"an unclosed substitution reference stays plain text, no warning at all",
			"unclosed |sub here\n",
			"<document>\n    <paragraph>\n        unclosed |sub here\n",
		},
		{
			"a role-prefixed unclosed backquote still ends up byte-identical to real docutils",
			"see :role:`unclosed here\n",
			"<document>\n    <paragraph>\n        see :role:\n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            `\n        unclosed here\n    <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"2\" line=\"1\" type=\"WARNING\">\n        <paragraph>\n            Inline interpreted text or phrase reference start-string without end-string.\n",
		},
		{
			"an unclosed marker in a section TITLE attaches to the section itself, not a trailing document section — byte-identical to real docutils including line",
			"Test *unclosed title\n=====================\n",
			"<document>\n    <section id=\"test-unclosed-title\" name=\"test *unclosed title\">\n        <title>\n            Test \n            <problematic id=\"problematic-1\" refid=\"system-message-1\">\n                *\n            unclosed title\n        <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"2\" line=\"1\" type=\"WARNING\">\n            <paragraph>\n                Inline emphasis start-string without end-string.\n",
		},
		{
			"an unclosed marker inside a nested construct (a list item) still attaches as a sibling of its own paragraph, not a trailing document section — line is omitted (parser.currentLine's own documented scope boundary: a list item's lines are a rebased sub-slice, not tracked back to an absolute document position), everything else byte-identical to real docutils",
			"- item with *unclosed here\n",
			"<document>\n    <bullet_list bullet=\"-\">\n        <list_item>\n            <paragraph>\n                item with \n                <problematic id=\"problematic-1\" refid=\"system-message-1\">\n                    *\n                unclosed here\n            <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"2\" type=\"WARNING\">\n                <paragraph>\n                    Inline emphasis start-string without end-string.\n",
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
