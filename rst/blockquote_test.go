package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestBlockQuoteNesting covers docutils' StringList.get_indented: a block
// quote's indent is discovered from the MINIMUM across every line in its
// (possibly variable-depth) run, not the first line's own indent — a real,
// previously-undetected bug (this project's own consumeIndentedBlock, a
// FIXED-first-line-indent extraction correct for a list item or field
// body, silently flattened a deeper-then-shallower run into sibling
// block_quotes instead of nesting the deeper one inside the shallower).
func TestBlockQuoteNesting(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a deeper-then-shallower run nests, not siblings",
			"Paragraph.\n\n        Deep.\n\n    Shallow.\n",
			"<document>\n    <paragraph>\n        Paragraph.\n    <block_quote>\n        <block_quote>\n            <paragraph>\n                Deep.\n        <paragraph>\n            Shallow.\n",
		},
		{
			"three levels nest correctly",
			"Top.\n\n            Level 3.\n\n        Level 2.\n\n    Level 1.\n",
			"<document>\n    <paragraph>\n        Top.\n    <block_quote>\n        <block_quote>\n            <block_quote>\n                <paragraph>\n                    Level 3.\n            <paragraph>\n                Level 2.\n        <paragraph>\n            Level 1.\n",
		},
		{
			"a uniform-depth run stays a single block_quote, not falsely nested",
			"Paragraph.\n\n    One.\n\n    Two.\n",
			"<document>\n    <paragraph>\n        Paragraph.\n    <block_quote>\n        <paragraph>\n            One.\n        <paragraph>\n            Two.\n",
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

// TestCheckAttributionShape unit-tests the shape check directly: real
// docutils rejects an attribution whose continuation lines don't all
// share ONE indent (verified against the foreign judge for the general
// principle — a real end-to-end Parse test for the rejection case itself
// isn't practical here, since once check_attribution rejects, the
// differently-indented line that caused it ALSO triggers real docutils'
// separate "Unexpected indentation" diagnostic, a genuinely different,
// already-out-of-scope feature this parser doesn't implement, so a
// full-document comparison would conflate the two).
func TestCheckAttributionShape(t *testing.T) {
	indented := []string{"Attribution one", "Attribution two", "   Attribution three"}
	if _, _, ok := checkAttributionShape(indented, 0); ok {
		t.Fatalf("expected inconsistent-indent continuation to reject the shape")
	}

	indented2 := []string{"Attribution one", "  Attribution two", "  Attribution three"}
	end, indent, ok := checkAttributionShape(indented2, 0)
	if !ok || end != 3 || indent != 2 {
		t.Fatalf("consistent 2-space continuation: end=%d indent=%d ok=%v, want end=3 indent=2 ok=true", end, indent, ok)
	}

	indented3 := []string{"Attribution one"}
	end3, indent3, ok3 := checkAttributionShape(indented3, 0)
	if !ok3 || end3 != 1 || indent3 != 0 {
		t.Fatalf("single-line attribution (no continuation at all): end=%d indent=%d ok=%v, want end=1 indent=0 ok=true", end3, indent3, ok3)
	}
}

// TestBlockQuoteAttribution covers docutils' split_attribution +
// check_attribution: a block quote's own trailing "-- text" / "--- text" /
// em-dash-prefixed line (preceded by a blank line, with any further lines
// sharing ONE consistent indent) becomes an <attribution>, splitting the
// surrounding indented region into a separate <block_quote> per
// attribution boundary — a construct this parser didn't recognize at all
// before, falling back to an ordinary trailing paragraph (or, once a
// second differently-indented continuation line was involved, actually
// misparsing as a <definition_list>).
func TestBlockQuoteAttribution(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a simple two-dash attribution",
			"Paragraph.\n\n   Block quote.\n\n   -- Attribution\n",
			"<document>\n    <paragraph>\n        Paragraph.\n    <block_quote>\n        <paragraph>\n            Block quote.\n        <attribution>\n            Attribution\n",
		},
		{
			"an em-dash attribution",
			"Paragraph.\n\n   Block quote.\n\n   — Attribution\n",
			"<document>\n    <paragraph>\n        Paragraph.\n    <block_quote>\n        <paragraph>\n            Block quote.\n        <attribution>\n            Attribution\n",
		},
		{
			"four dashes is not an attribution marker, stays a paragraph",
			"Paragraph.\n\n   Block quote.\n\n   ---- not attribution\n",
			"<document>\n    <paragraph>\n        Paragraph.\n    <block_quote>\n        <paragraph>\n            Block quote.\n        <paragraph>\n            ---- not attribution\n",
		},
		{
			// Still not an attribution -- but "--" IS a short adornment, so
			// it also draws docutils' "too short to be an overline or
			// transition" INFO before being treated as ordinary text.
			// Checked against the reference on this exact input; the old
			// expectation here simply predated that diagnostic.
			"a bare dash marker with nothing after it is not an attribution",
			"Paragraph.\n\n   Block quote.\n\n   --\n",
			"<document>\n    <paragraph>\n        Paragraph.\n    <block_quote>\n        <paragraph>\n            Block quote.\n        <system_message level=\"1\" line=\"5\" type=\"INFO\">\n            <paragraph>\n                Unexpected possible title overline or transition.\n                Treating it as ordinary text because it's so short.\n        <paragraph>\n            --\n",
		},
		{
			"an attribution at the outer level of a nested block quote",
			"Paragraph.\n\n        Deep.\n\n    -- Attribution at outer level\n",
			"<document>\n    <paragraph>\n        Paragraph.\n    <block_quote>\n        <block_quote>\n            <paragraph>\n                Deep.\n        <attribution>\n            Attribution at outer level\n",
		},
		{
			"two block quotes each with their own attribution become two siblings",
			"Paragraph.\n\n   Block quote 1.\n\n   -- Attribution 1\n\n   Block quote 2.\n\n   --Attribution 2\n",
			"<document>\n    <paragraph>\n        Paragraph.\n    <block_quote>\n        <paragraph>\n            Block quote 1.\n        <attribution>\n            Attribution 1\n    <block_quote>\n        <paragraph>\n            Block quote 2.\n        <attribution>\n            Attribution 2\n",
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
