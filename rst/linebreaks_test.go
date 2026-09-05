package rst

import (
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestPythonLineBoundaries pins line splitting to every boundary Python's
// str.splitlines() recognizes, which is what docutils' string2lines is
// built on -- not just "\n". A corpus fixture wraps markup around a
// U+2028 ("* LINE SEPARATOR *") precisely to check it is a line
// break rather than ordinary text inside one line.
func TestPythonLineBoundaries(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"U+2028 LINE SEPARATOR splits a line",
			"a b\n",
			"<document>\n    <paragraph>\n        a\n        b\n",
		},
		{
			"U+2029 PARAGRAPH SEPARATOR splits a line too",
			"a b\n",
			"<document>\n    <paragraph>\n        a\n        b\n",
		},
		{
			// The reason it matters: the two "*" end up on DIFFERENT
			// lines, so nothing spans the boundary. The exact shape below
			// (an empty bullet list from the lone "*", then two
			// diagnostics) is byte-for-byte what the reference produces
			// for this input -- an earlier version of this test guessed
			// "three lines of plain text" and was simply wrong.
			"markup does not span a U+2028 boundary",
			"*\u2028emphasis\u2028*\n",
			"<document>\n    <bullet_list bullet=\"*\">\n        <list_item>\n    <system_message level=\"2\" line=\"2\" type=\"WARNING\">\n        <paragraph>\n            Bullet list ends without a blank line; unexpected unindent.\n    <system_message level=\"1\" line=\"3\" type=\"INFO\">\n        <paragraph>\n            Possible title underline, too short for the title.\n            Treating it as ordinary text because it's so short.\n    <paragraph>\n        emphasis\n        *\n",
		},
		{
			// A NO-BREAK SPACE is whitespace but NOT a line boundary, so
			// this stays one line -- and the markup still fails, for the
			// different reason that a start-string may not be followed by
			// whitespace.
			"a no-break space is not a line boundary",
			"* text *\n",
			"<document>\n    <paragraph>\n        * text *\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := doctree.Dump(Parse(tc.source)); got != tc.want {
				t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
}

// TestDoctestBlockSwallowsIndentedOutput covers Body.doctest reading its
// block with get_text_block() and NO flush_left argument: it stops at a
// BLANK line and nothing else, because the indented lines after a prompt
// are the interpreter's own OUTPUT and belong to the block. Stopping at
// the first indented line instead handed that output to a spurious block
// quote.
func TestDoctestBlockSwallowsIndentedOutput(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"indented output stays inside the doctest block",
			"Paragraph.\n\n>>> print(\"    Indented output.\")\n    Indented output.\n",
			"<document>\n    <paragraph>\n        Paragraph.\n    <doctest_block>\n        >>> print(\"    Indented output.\")\n            Indented output.\n",
		},
		{
			"the same inside a block quote, where the whole block is indented",
			"Paragraph.\n\n    >>> print(\"    Indented block & output.\")\n        Indented block & output.\n",
			"<document>\n    <paragraph>\n        Paragraph.\n    <block_quote>\n        <doctest_block>\n            >>> print(\"    Indented block & output.\")\n                Indented block & output.\n",
		},
		{
			// The control: a BLANK line does end it.
			"a blank line ends the doctest block",
			">>> code\n\nAfter.\n",
			"<document>\n    <doctest_block>\n        >>> code\n    <paragraph>\n        After.\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := doctree.Dump(Parse(tc.source)); got != tc.want {
				t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
}
