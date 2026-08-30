package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// Expected trees below were cross-checked against reference docutils
// 0.23 (`publish_string(..., writer_name='pseudoxml',
// settings_overrides={'doctitle_xform': False})`) for element shape;
// this project's own doctree.Dump format is used for comparison since
// ids/names/source attributes are not yet implemented (see the package
// doc's SCOPE note). A field list at the very start of a document is
// compared against docutils with `docinfo_xform: False` too — real
// docutils otherwise promotes it to a typed `<docinfo>` node, a
// transform this parser does not implement (same category of gap as
// per-directive semantics). Two more known, documented divergences: a
// directive is captured structurally (name/arguments/raw content)
// rather than dispatched to semantics (`.. note::` does NOT become a
// real `<note>` admonition here), and an unresolved reference stays a
// bare reference node instead of being rewritten to `problematic` with
// an appended system-message section.
func TestParse(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "sections, paragraph with inline markup, lists",
			source: "Section Title\n=============\n\nSubsection\n----------\n\nA paragraph with *emphasis* and ``literal`` text.\n\n- a bullet\n- list\n\n1. an enumerated\n2. list\n",
			want:   "<document>\n    <section>\n        <title>\n            Section Title\n        <section>\n            <title>\n                Subsection\n            <paragraph>\n                A paragraph with \n                <emphasis>\n                    emphasis\n                 and \n                <literal>\n                    literal\n                 text.\n            <bullet_list bullet=\"-\">\n                <list_item>\n                    <paragraph>\n                        a bullet\n                <list_item>\n                    <paragraph>\n                        list\n            <enumerated_list>\n                <list_item>\n                    <paragraph>\n                        an enumerated\n                <list_item>\n                    <paragraph>\n                        list\n",
		},
		{
			name:   "paragraph, block quote with two paragraphs, transition, paragraph",
			source: "First paragraph.\n\n    An indented block quote.\n\n    Second line of quote.\n\n----\n\nLast paragraph.\n",
			want:   "<document>\n    <paragraph>\n        First paragraph.\n    <block_quote>\n        <paragraph>\n            An indented block quote.\n        <paragraph>\n            Second line of quote.\n    <transition>\n    <paragraph>\n        Last paragraph.\n",
		},
		{
			name:   "enumerated list item containing a nested bullet list",
			source: "1. outer item one\n\n   - nested bullet a\n   - nested bullet b\n\n2. outer item two\n",
			want:   "<document>\n    <enumerated_list>\n        <list_item>\n            <paragraph>\n                outer item one\n            <bullet_list bullet=\"-\">\n                <list_item>\n                    <paragraph>\n                        nested bullet a\n                <list_item>\n                    <paragraph>\n                        nested bullet b\n        <list_item>\n            <paragraph>\n                outer item two\n",
		},
		{
			name:   "block quote containing a bullet list and a second paragraph",
			source: "Intro.\n\n    - item in quote\n    - second item\n\n    Another paragraph in the same quote.\n",
			want:   "<document>\n    <paragraph>\n        Intro.\n    <block_quote>\n        <bullet_list bullet=\"-\">\n            <list_item>\n                <paragraph>\n                    item in quote\n            <list_item>\n                <paragraph>\n                    second item\n        <paragraph>\n            Another paragraph in the same quote.\n",
		},
		{
			name:   "list item with two paragraphs and a nested enumerated list",
			source: "1. outer item\n\n   Second paragraph of outer item.\n\n   1. nested enumerated a\n   2. nested enumerated b\n",
			want:   "<document>\n    <enumerated_list>\n        <list_item>\n            <paragraph>\n                outer item\n            <paragraph>\n                Second paragraph of outer item.\n            <enumerated_list>\n                <list_item>\n                    <paragraph>\n                        nested enumerated a\n                <list_item>\n                    <paragraph>\n                        nested enumerated b\n",
		},
		{
			name:   "comment, directive, hyperlink target with reference resolution, literal block",
			source: "Intro paragraph.\n\n.. This is a comment.\n   Second comment line.\n\n.. note::\n\n   This is directive content.\n   Second line.\n\nSee `Example`_ for details.\n\n.. _Example: https://example.com\n\nHere is a code sample::\n\n    def f():\n        return 1\n\nDone.\n",
			want:   "<document>\n    <paragraph>\n        Intro paragraph.\n    <comment>\n        This is a comment.\n        Second comment line.\n    <directive name=\"note\">\n        This is directive content.\n        Second line.\n    <paragraph>\n        See \n        <reference refname=\"Example\" refuri=\"https://example.com\">\n            Example\n         for details.\n    <target name=\"example\" refuri=\"https://example.com\">\n    <paragraph>\n        Here is a code sample:\n    <literal_block>\n        def f():\n            return 1\n    <paragraph>\n        Done.\n",
		},
		{
			name:   "unresolved reference, empty comment, plain comment, directive with no content",
			source: "An unresolved `Nowhere`_ reference.\n\n..\n\n.. plain comment no directive shape\n\n.. figure::\n",
			want:   "<document>\n    <paragraph>\n        An unresolved \n        <reference refname=\"Nowhere\">\n            Nowhere\n         reference.\n    <comment>\n    <comment>\n        plain comment no directive shape\n    <directive name=\"figure\">\n",
		},
		{
			name:   "list item containing a comment and a literal block",
			source: "- item one\n\n  .. a comment inside a list item\n\n  a literal sample::\n\n      x = 1\n\n- item two\n",
			want:   "<document>\n    <bullet_list bullet=\"-\">\n        <list_item>\n            <paragraph>\n                item one\n            <comment>\n                a comment inside a list item\n            <paragraph>\n                a literal sample:\n            <literal_block>\n                x = 1\n        <list_item>\n            <paragraph>\n                item two\n",
		},
		{
			name:   "field list with a continuation line indented less than the marker column, definition list",
			source: ":author: Jane Doe\n:version: 1.0\n:date: 2026-08-30\n  continuation line for date\n\nTerm one\n    Definition of term one.\n\nTerm two\n    First paragraph of definition two.\n\n    Second paragraph of definition two.\n\n    - a nested bullet inside the definition\n",
			want:   "<document>\n    <field_list>\n        <field>\n            <field_name>\n                author\n            <field_body>\n                <paragraph>\n                    Jane Doe\n        <field>\n            <field_name>\n                version\n            <field_body>\n                <paragraph>\n                    1.0\n        <field>\n            <field_name>\n                date\n            <field_body>\n                <paragraph>\n                    2026-08-30\n                    continuation line for date\n    <definition_list>\n        <definition_list_item>\n            <term>\n                Term one\n            <definition>\n                <paragraph>\n                    Definition of term one.\n        <definition_list_item>\n            <term>\n                Term two\n            <definition>\n                <paragraph>\n                    First paragraph of definition two.\n                <paragraph>\n                    Second paragraph of definition two.\n                <bullet_list bullet=\"-\">\n                    <list_item>\n                        <paragraph>\n                            a nested bullet inside the definition\n",
		},
		{
			name:   "a blank line after a would-be term prevents definition-list detection",
			source: "Not a term because next line is blank.\n\nTerm with no definition body next line\n\nA regular paragraph that follows.\n",
			want:   "<document>\n    <paragraph>\n        Not a term because next line is blank.\n    <paragraph>\n        Term with no definition body next line\n    <paragraph>\n        A regular paragraph that follows.\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if got != tc.want {
				t.Errorf("mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, tc.want)
			}
		})
	}
}

func TestInline(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"strong", "a **bold** word", "a \n<strong>\n    bold\n word\n"},
		{"emphasis", "an *italic* word", "an \n<emphasis>\n    italic\n word\n"},
		{"literal", "some ``code()`` here", "some \n<literal>\n    code()\n here\n"},
		{"named reference", "see `Section`_ for details", "see \n<reference refname=\"Section\">\n    Section\n for details\n"},
		{"no nested markup inside strong", "**a *b* c**", "<strong>\n    a *b* c\n"},
		{"unmatched marker stays literal", "2 * 3 = 6", "2 * 3 = 6\n"},
		{"backslash escape suppresses markup", "\\*not emphasis\\*", "*not emphasis*\n"},
		{"anonymous reference", "see `some text`__ end", "see \n<reference anonymous=\"true\" refname=\"some text\">\n    some text\n end\n"},
		{"unclosed marker falls back to plain text", "an *unclosed emphasis stays plain", "an *unclosed emphasis stays plain\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes := parseInline(tc.text)
			var b strings.Builder
			for _, n := range nodes {
				b.WriteString(doctree.Dump(n))
			}
			if b.String() != tc.want {
				t.Errorf("mismatch:\n--- got ---\n%s\n--- want ---\n%s", b.String(), tc.want)
			}
		})
	}
}
