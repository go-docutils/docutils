package rst

import (
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestParagraphTextIsRstripped reads the paragraph's text node VALUE rather
// than its pseudoxml, which is the whole point of the case.
//
// Real docutils builds a paragraph's text as "'\n'.join(lines).rstrip()"
// (states.py, Text.paragraph), and rstrip takes every kind of trailing
// whitespace. This parser trimmed only " ", which left the NEWLINE on a
// paragraph whose last line had been taken away from it -- and that is exactly
// what happens to the "::" of a literal block written on a line of its own,
// where docutils does "data[:-3].rstrip()".
//
// Every other test of this construct in the package compares PSEUDOXML, which
// prints a text node's value indented under its element and cannot show a
// trailing newline at all. So the corpus could read 1564/1564 against the
// reference with 12 files' text nodes differing: "identical pseudoxml" is not
// "identical trees". The reference was asked for the value directly
// (publish_doctree, then the node's own astext) and answers
// 'In a Unicode string,' for PEP 223's paragraph.
func TestParagraphTextIsRstripped(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			// TWO lines, because a "::" under a ONE-line paragraph is a
			// title-underline candidate in the reference and comes with a
			// "Possible title underline, too short for the title." message --
			// a different construct, and asking for the minimal witness got
			// that one instead. PEP 223's real paragraph is this shape.
			"the :: of a literal block on a line of its OWN takes the newline with it",
			"In a Unicode string,\nand more text,\n::\n\n   \\xij\n",
			"In a Unicode string,\nand more text,",
		},
		{
			"trailing spaces on a paragraph's last line",
			"a paragraph with trailing spaces   \n\nnext\n",
			"a paragraph with trailing spaces",
		},
		{
			// Not this function's doing, and asserted here because I expected
			// the spaces to SURVIVE: docutils rstrips every line when it reads
			// the source (statemachine.string2lines), so a continuation line's
			// trailing spaces are gone before any of this runs, and the
			// reference answers 'first line\nsecond line'. Checked against it
			// rather than reasoned about.
			"trailing spaces on a CONTINUATION line are stripped by the READER, not here",
			"first line   \nsecond line\n\nnext\n",
			"first line\nsecond line",
		},
		{
			"a space before the :: takes the space with it, same rule one axis over",
			"a paragraph ::\n\n   code\n",
			"a paragraph",
		},
		{
			"word:: keeps ONE colon and nothing is stripped",
			"a paragraph::\n\n   code\n",
			"a paragraph:",
		},
		{
			"an ordinary paragraph is untouched",
			"just text\non two lines\n",
			"just text\non two lines",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := ParseWithOptions(c.source, DefaultOptions())
			if doc == nil {
				t.Fatal("ParseWithOptions returned nil")
			}
			got, ok := firstParagraphText(doc)
			if !ok {
				t.Fatalf("no paragraph with a text child in:\n%s", doctree.Dump(doc))
			}
			if got != c.want {
				t.Errorf("text node = %q, want %q", got, c.want)
			}
		})
	}
}

func firstParagraphText(n doctree.Node) (string, bool) {
	el, ok := n.(*doctree.Element)
	if !ok {
		return "", false
	}
	if el.Tag == doctree.TagParagraph && len(el.Children) > 0 {
		if t, ok := el.Children[0].(*doctree.Text); ok {
			return t.Data, true
		}
	}
	for _, ch := range el.Children {
		if s, ok := firstParagraphText(ch); ok {
			return s, true
		}
	}
	return "", false
}
