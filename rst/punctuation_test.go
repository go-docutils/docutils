package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestPunctuationCharacterClasses spot-checks isOpener/isCloser/
// isDelimiterChar/isClosingDelimiter against docutils.utils.punctuation_chars
// — the full sets were verified exactly (every Unicode code point) against a
// live Python reference during development; these are the handful of cases
// most likely to regress if the tables are ever hand-edited again.
func TestPunctuationCharacterClasses(t *testing.T) {
	openers := []rune{'(', '[', '{', '"', '\'', '«', '‘', '“', '〈', '「'}
	for _, r := range openers {
		if !isOpener(r) {
			t.Errorf("isOpener(%q) = false, want true", r)
		}
	}
	closers := []rune{')', ']', '}', '"', '\'', '»', '’', '”', '〉', '」'}
	for _, r := range closers {
		if !isCloser(r) {
			t.Errorf("isCloser(%q) = false, want true", r)
		}
	}
	// A closer must NOT be classified as an opener, and vice versa — the
	// exact asymmetry this whole feature exists to enforce (unlike
	// unicode.IsPunct, which conflates both).
	if isOpener(')') {
		t.Error("isOpener(')') = true, want false — a closer is not an opener")
	}
	if isCloser('(') {
		t.Error("isCloser('(') = true, want false — an opener is not a closer")
	}
	delimiters := []rune{'-', '/', ':', '–', '—', '…', '‰'}
	for _, r := range delimiters {
		if !isDelimiterChar(r) {
			t.Errorf("isDelimiterChar(%q) = false, want true", r)
		}
	}
	closingDelims := []rune{'.', ',', ';', '!', '?', '\\'}
	for _, r := range closingDelims {
		if !isClosingDelimiter(r) {
			t.Errorf("isClosingDelimiter(%q) = false, want true", r)
		}
	}
	// An ordinary letter or digit is none of the above.
	for _, r := range []rune{'a', 'Z', '5', '_'} {
		if isOpener(r) || isCloser(r) || isDelimiterChar(r) || isClosingDelimiter(r) {
			t.Errorf("%q classified as punctuation, want none of the four sets", r)
		}
	}
}

// TestQuotedStart covers punctuation_chars.match_chars, called from real
// docutils' Inliner.quoted_start — verified against the foreign judge for
// every case here, including the one real quirk in the reference data
// itself: U+301D appears twice in docutils' own openers string, pairing
// with two different closers, but only the FIRST pairing is reachable
// (Python's str.index finds only the first occurrence).
func TestQuotedStart(t *testing.T) {
	cases := []struct {
		c1, c2 rune
		want   bool
	}{
		{'(', ')', true}, {'(', '(', false}, {'"', '"', true}, {'\'', '\'', true},
		{'‘', '’', true}, {'‘', '‚', true}, {'‚', '‘', true}, {'‚', '’', true},
		{'“', '”', true}, {'“', '„', true}, {'„', '“', true}, {'„', '”', true},
		{'»', '»', true}, {'«', '»', true}, {'›', '›', true}, {'‹', '›', true},
		{'x', 'y', false}, {'(', 'x', false},
		{'〝', '〞', true}, {'〝', '〟', false},
	}
	for _, c := range cases {
		if got := quotedStart(c.c1, c.c2); got != c.want {
			t.Errorf("quotedStart(%q, %q) = %v, want %v", c.c1, c.c2, got, c.want)
		}
	}
}

// TestInlineMarkupBoundaries covers docutils' start_string_prefix +
// end_string_suffix (Inliner.init_customizations, ported in
// validStartBoundary/findClose), each case verified against the foreign
// judge — replacing this parser's earlier, much simpler unicode.IsPunct
// approximation, which accepted an opening and a closing bracket/quote
// identically on either side of a marker (real docutils does not: it
// distinguishes an OPENER/DELIMITER, valid before a start-string, from a
// CLOSER/CLOSING-DELIMITER, valid after an end-string, and rejects the
// reverse on either side).
func TestInlineMarkupBoundaries(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a delimiter (slash) on both sides is a valid boundary",
			"/*emphasis*/, more text\n",
			"<document>\n    <paragraph>\n        /\n        <emphasis>\n            emphasis\n        /, more text\n",
		},
		{
			"open/close bracket pairs around the whole markup are valid",
			"(*emphasis*), more text\n",
			"<document>\n    <paragraph>\n        (\n        <emphasis>\n            emphasis\n        ), more text\n",
		},
		{
			"a closing delimiter (period) right after the end-string is valid",
			"*emphasis*. more text\n",
			"<document>\n    <paragraph>\n        <emphasis>\n            emphasis\n        . more text\n",
		},
		{
			"a CLOSING bracket right before the start-string is REJECTED, not a valid start",
			"word)*emphasis* more\n",
			"<document>\n    <paragraph>\n        word)*emphasis* more\n",
		},
		{
			"an OPENING bracket right after the end-string is REJECTED, not a valid end",
			"*emphasis*(more\n",
			"<document>\n    <paragraph>\n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            *\n        emphasis*(more\n    <section class=\"system-messages\">\n        <title>\n            Docutils System Messages\n        <system_message backref=\"problematic-1\" id=\"system-message-1\">\n            <paragraph>\n                Inline emphasis start-string without end-string.\n",
		},
		{
			"an alphanumeric character immediately before the start-string is invalid",
			"x*2* or 2*x*\n",
			"<document>\n    <paragraph>\n        x*2* or 2*x*\n",
		},
		{
			"a bare marker with nothing at all after it is not even an attempt",
			"text ending in a marker *\n",
			"<document>\n    <paragraph>\n        text ending in a marker *\n",
		},
		{
			"a quoted-start sandwiched pair, opener then its own closer with nothing between, is rejected",
			"(*)text\n",
			"<document>\n    <paragraph>\n        (*)text\n",
		},
		{
			"a backslash-escaped space right after the end-string is a valid boundary and is dropped from the text",
			"*emphasis*\\ (closing delimiters)\n",
			"<document>\n    <paragraph>\n        <emphasis>\n            emphasis\n        (closing delimiters)\n",
		},
		{
			"a backslash-escaped space right before the start-string is NOT a valid boundary on its own (still needs whitespace/opener/delimiter behind that)",
			"\\*args or * (escaped, whitespace behind start-string)\n",
			"<document>\n    <paragraph>\n        *args or * (escaped, whitespace behind start-string)\n",
		},
		{
			"a named reference by backquote resolves through the trailing underscore, not a problematic",
			"see `Section`_ for details\n\nSection\n=======\n",
			"<document>\n    <paragraph>\n        see \n        <reference name=\"Section\" refname=\"section\" refuri=\"#section\">\n            Section\n         for details\n    <section id=\"section\" name=\"section\">\n        <title>\n            Section\n",
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
