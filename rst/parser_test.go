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
// doc's SCOPE note).
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
