package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestLiteralBlocks covers docutils.states.Text.literal_block and its
// escaping/indentation quirks (test_literal_blocks.py[indented_literal_blocks],
// read directly and verified against the foreign judge) — a paragraph
// ending in an unescaped "::" marks what follows as a literal block
// rather than a block quote, with real docutils' own diagnostics for
// every way that can go wrong: a continuation line breaking the
// paragraph short because it's indented ("Unexpected indentation."), an
// indented block that ends on a non-blank unindented line instead of a
// blank one ("...ends without a blank line; unexpected unindent."), no
// indentation at all following the "::" ("Literal block expected; none
// found."), and the "::" itself being escaped (an ODD number of
// backslashes immediately before it — states.py's own
// "(?<!\\)(\\\\)*::$" — means no literal block and no "::" reduction at
// all, ported in consumeParagraph).
func TestLiteralBlocks(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a continuation line more indented than the paragraph breaks it short with an error, the literal block still follows",
			"A paragraph\non more than\none line::\n    A literal block\n    with no blank line above.\n",
			"<document>\n    <paragraph>\n        A paragraph\n        on more than\n        one line:\n    <system_message level=\"3\" line=\"4\" type=\"ERROR\">\n        <paragraph>\n            Unexpected indentation.\n    <literal_block>\n        A literal block\n        with no blank line above.\n",
		},
		{
			"a literal block ending on a non-blank unindented line warns about the missing blank line",
			"A paragraph::\n\n    A literal block.\nno blank line\n",
			"<document>\n    <paragraph>\n        A paragraph:\n    <literal_block>\n        A literal block.\n    <system_message level=\"2\" line=\"4\" type=\"WARNING\">\n        <paragraph>\n            Literal block ends without a blank line; unexpected unindent.\n    <paragraph>\n        no blank line\n",
		},
		{
			"an escaped backslash before :: still triggers a literal block, reduced to a single trailing colon",
			"A paragraph\\\\::\n\n    A literal block.\n",
			"<document>\n    <paragraph>\n        A paragraph\\:\n    <literal_block>\n        A literal block.\n",
		},
		{
			"a single escaping backslash right before :: means only ONE real colon, no literal block, no reduction",
			"A paragraph\\::\n\n    Not a literal block.\n",
			"<document>\n    <paragraph>\n        A paragraph::\n    <block_quote>\n        <paragraph>\n            Not a literal block.\n",
		},
		{
			"a literal block with no following text at all except an indented block, minimal case",
			"\\\\::\n\n    A literal block.\n",
			"<document>\n    <paragraph>\n        \\:\n    <literal_block>\n        A literal block.\n",
		},
		{
			"the same minimal case with the single-backslash (non-triggering) escape",
			"\\::\n\n    Not a literal block.\n",
			"<document>\n    <paragraph>\n        ::\n    <block_quote>\n        <paragraph>\n            Not a literal block.\n",
		},
		{
			"a paragraph ending in :: with a following block that is not indented at all warns instead of silently skipping",
			"A paragraph::\n\nNot a literal block.\n",
			"<document>\n    <paragraph>\n        A paragraph:\n    <system_message level=\"2\" line=\"3\" type=\"WARNING\">\n        <paragraph>\n            Literal block expected; none found.\n    <paragraph>\n        Not a literal block.\n",
		},
		{
			"a single line whose own indentation is inconsistent within itself still dedents by the block's true minimum",
			"A paragraph::\n\n    A wonky literal block.\n  Literal line 2.\n\n    Literal line 3.\n",
			"<document>\n    <paragraph>\n        A paragraph:\n    <literal_block>\n          A wonky literal block.\n        Literal line 2.\n        \n          Literal line 3.\n",
		},
		{
			"a paragraph ending in :: with nothing at all following it (real EOF) still warns",
			"EOF, even though a literal block is indicated::\n",
			"<document>\n    <paragraph>\n        EOF, even though a literal block is indicated:\n    <system_message level=\"2\" line=\"2\" type=\"WARNING\">\n        <paragraph>\n            Literal block expected; none found.\n",
		},
		{
			"an unindented literal block quoted with a punctuation character on every line",
			"A paragraph::\n\n> A literal block.\n> Line 2.\n",
			"<document>\n    <paragraph>\n        A paragraph:\n    <literal_block>\n        > A literal block.\n        > Line 2.\n",
		},
		{
			"a quoted literal block followed by an indented line errors and yields a block_quote sibling",
			"A paragraph::\n\n> A literal block.\n  Indented line.\n",
			"<document>\n    <paragraph>\n        A paragraph:\n    <literal_block>\n        > A literal block.\n    <system_message level=\"3\" line=\"4\" type=\"ERROR\">\n        <paragraph>\n            Unexpected indentation.\n    <block_quote>\n        <paragraph>\n            Indented line.\n",
		},
		{
			"a quoted literal block followed by a differently-quoted or unquoted line errors as inconsistent",
			"A paragraph::\n\n> A literal block.\n$ Inconsistent line.\n",
			"<document>\n    <paragraph>\n        A paragraph:\n    <literal_block>\n        > A literal block.\n    <system_message level=\"3\" line=\"4\" type=\"ERROR\">\n        <paragraph>\n            Inconsistent literal block quoting.\n    <paragraph>\n        $ Inconsistent line.\n",
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
