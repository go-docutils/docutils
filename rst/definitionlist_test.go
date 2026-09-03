package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestDefinitionListDiagnosticsAndClassifiers covers three docutils
// mechanisms (states.py, read directly, all verified against
// test_definition_lists.py's own corpus fixtures): the "Definition list
// ends without a blank line; unexpected unindent." warning
// (Text.indent's own unindent_warning, the same shared shape footnotes/
// citations and line blocks already have); the "Blank line missing
// before literal block..." INFO a term ending in "::" gets
// (Text.definition_list_item); and a term's own "term : classifier"
// splitting (Text.term/classifier_delimiter) — including the one real
// escaping subtlety: a BACKSLASH-ESCAPED colon never counts as the
// delimiter, even with real spaces on both sides.
func TestDefinitionListDiagnosticsAndClassifiers(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a definition list interrupted by a non-blank line warns about the missing blank line",
			"term\n  definition\nno blank line\n",
			"<document>\n    <definition_list>\n        <definition_list_item>\n            <term>\n                term\n            <definition>\n                <paragraph>\n                    definition\n    <system_message level=\"2\" line=\"3\" type=\"WARNING\">\n        <paragraph>\n            Definition list ends without a blank line; unexpected unindent.\n    <paragraph>\n        no blank line\n",
		},
		{
			"two chained items with no blank line between them are fine; only the FINAL abrupt end warns",
			"term 1\n  definition 1 (no blank line below)\nterm 2\n  definition 2\nNo blank line after the definition list.\n",
			"<document>\n    <definition_list>\n        <definition_list_item>\n            <term>\n                term 1\n            <definition>\n                <paragraph>\n                    definition 1 (no blank line below)\n        <definition_list_item>\n            <term>\n                term 2\n            <definition>\n                <paragraph>\n                    definition 2\n    <system_message level=\"2\" line=\"5\" type=\"WARNING\">\n        <paragraph>\n            Definition list ends without a blank line; unexpected unindent.\n    <paragraph>\n        No blank line after the definition list.\n",
		},
		{
			"a real blank line right before the interrupting text is a clean finish, no warning",
			"term\n  definition\n\nA real paragraph.\n",
			"<document>\n    <definition_list>\n        <definition_list_item>\n            <term>\n                term\n            <definition>\n                <paragraph>\n                    definition\n    <paragraph>\n        A real paragraph.\n",
		},
		{
			"a term ending in :: with no blank line before its body gets an INFO diagnostic",
			"A paragraph::\n    A literal block without a blank line first?\n",
			"<document>\n    <definition_list>\n        <definition_list_item>\n            <term>\n                A paragraph::\n            <definition>\n                <system_message level=\"1\" line=\"2\" type=\"INFO\">\n                    <paragraph>\n                        Blank line missing before literal block (after the \"::\")? Interpreted as a definition list item.\n                <paragraph>\n                    A literal block without a blank line first?\n",
		},
		{
			"a colon surrounded by real spaces splits the term into a classifier",
			"Term : classifier\n    The ' : ' indicates a classifier in\n    definition list item terms only.\n",
			"<document>\n    <definition_list>\n        <definition_list_item>\n            <term>\n                Term\n            <classifier>\n                classifier\n            <definition>\n                <paragraph>\n                    The ' : ' indicates a classifier in\n                    definition list item terms only.\n",
		},
		{
			"repeated ' : ' delimiters open multiple classifiers",
			"Term : classifier one  :  classifier two\n    Definition\n",
			"<document>\n    <definition_list>\n        <definition_list_item>\n            <term>\n                Term\n            <classifier>\n                classifier one\n            <classifier>\n                classifier two\n            <definition>\n                <paragraph>\n                    Definition\n",
		},
		{
			"no space before the colon: not a classifier delimiter at all",
			"Term: not a classifier\n    Because there's no space before the colon.\n",
			"<document>\n    <definition_list>\n        <definition_list_item>\n            <term>\n                Term: not a classifier\n            <definition>\n                <paragraph>\n                    Because there's no space before the colon.\n",
		},
		{
			"no space after the colon: not a classifier delimiter at all",
			"Term :not a classifier\n    Because there's no space after the colon.\n",
			"<document>\n    <definition_list>\n        <definition_list_item>\n            <term>\n                Term :not a classifier\n            <definition>\n                <paragraph>\n                    Because there's no space after the colon.\n",
		},
		{
			"a backslash-escaped colon, even with real spaces around it, never counts as a classifier delimiter",
			"Term \\: not a classifier\n    Because the colon is escaped.\n",
			"<document>\n    <definition_list>\n        <definition_list_item>\n            <term>\n                Term : not a classifier\n            <definition>\n                <paragraph>\n                    Because the colon is escaped.\n",
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
