package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

func TestFootnoteSymbolLabel(t *testing.T) {
	cases := map[int]string{
		0: "*", 1: "†", 5: "#", 9: "♣", // first pass through all 10
		10: "**", 11: "††", // second pass: doubled
		20: "***", // third pass: tripled
	}
	for i, want := range cases {
		if got := footnoteSymbolLabel(i); got != want {
			t.Errorf("footnoteSymbolLabel(%d) = %q, want %q", i, got, want)
		}
	}
}

func TestFootnoteNumbering(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"an unnamed auto footnote is numbered and its reference gets a working refname + visible number",
			"An auto footnote [#]_.\n\n.. [#] Text.\n",
			"<document>\n    <paragraph>\n        An auto footnote \n        <footnote_reference auto=\"1\" id=\"footnote-reference-1\" refname=\"footnote-1\">\n            1\n        .\n    <footnote auto=\"1\" id=\"footnote-1\" name=\"footnote-1\">\n        <label>\n            1\n        <paragraph>\n            Text.\n",
		},
		{
			"an explicit numeric footnote makes auto-numbering skip that number",
			"See footnote [1]_, an auto footnote [#]_.\n\n.. [1] Manual.\n\n.. [#] Auto.\n",
			"<document>\n    <paragraph>\n        See footnote \n        <footnote_reference id=\"footnote-reference-1\" refname=\"1\">\n            1\n        , an auto footnote \n        <footnote_reference auto=\"1\" id=\"footnote-reference-2\" refname=\"footnote-1\">\n            2\n        .\n    <footnote id=\"footnote-1\" name=\"1\">\n        <label>\n            1\n        <paragraph>\n            Manual.\n    <footnote auto=\"1\" id=\"footnote-2\" name=\"footnote-1\">\n        <label>\n            2\n        <paragraph>\n            Auto.\n",
		},
		{
			"a named auto footnote shares the same numbering sequence as unnamed ones",
			"First [#]_, named [#note]_.\n\n.. [#] Body one.\n\n.. [#note] Body two.\n",
			"<document>\n    <paragraph>\n        First \n        <footnote_reference auto=\"1\" id=\"footnote-reference-1\" refname=\"footnote-1\">\n            1\n        , named \n        <footnote_reference auto=\"1\" id=\"footnote-reference-2\" refname=\"note\">\n            2\n        .\n    <footnote auto=\"1\" id=\"footnote-1\" name=\"footnote-1\">\n        <label>\n            1\n        <paragraph>\n            Body one.\n    <footnote auto=\"1\" id=\"note\" name=\"note\">\n        <label>\n            2\n        <paragraph>\n            Body two.\n",
		},
		{
			"multiple unnamed auto footnotes are matched positionally in document order",
			"First [#]_, second [#]_.\n\n.. [#] One.\n.. [#] Two.\n",
			"<document>\n    <paragraph>\n        First \n        <footnote_reference auto=\"1\" id=\"footnote-reference-1\" refname=\"footnote-1\">\n            1\n        , second \n        <footnote_reference auto=\"1\" id=\"footnote-reference-2\" refname=\"footnote-2\">\n            2\n        .\n    <footnote auto=\"1\" id=\"footnote-1\" name=\"footnote-1\">\n        <label>\n            1\n        <paragraph>\n            One.\n    <footnote auto=\"1\" id=\"footnote-2\" name=\"footnote-2\">\n        <label>\n            2\n        <paragraph>\n            Two.\n",
		},
		{
			"symbol footnotes get their own separate sequence from the symbols table",
			"First [*]_, second [*]_.\n\n.. [*] One.\n.. [*] Two.\n",
			"<document>\n    <paragraph>\n        First \n        <footnote_reference auto=\"*\" id=\"footnote-reference-1\" refname=\"footnote-1\">\n            *\n        , second \n        <footnote_reference auto=\"*\" id=\"footnote-reference-2\" refname=\"footnote-2\">\n            †\n        .\n    <footnote auto=\"*\" id=\"footnote-1\" name=\"footnote-1\">\n        <label>\n            *\n        <paragraph>\n            One.\n    <footnote auto=\"*\" id=\"footnote-2\" name=\"footnote-2\">\n        <label>\n            †\n        <paragraph>\n            Two.\n",
		},
		{
			"a reference with no matching auto footnote at all stays unresolved, not a crash",
			"An orphan auto reference [#]_.\n",
			"<document>\n    <paragraph>\n        An orphan auto reference \n        <footnote_reference auto=\"1\" id=\"footnote-reference-1\">\n        .\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(parseNumberingFootnotes(tc.source))
			if strings.TrimRight(got, "\n") != strings.TrimRight(tc.want, "\n") {
				t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
}

// parseNumberingFootnotes parses with Options.NumberAutoFootnotes on. It
// is OFF by default -- docutils assigns auto-footnote numbers and symbols
// in transforms.references.Footnotes, so its bare parse leaves an auto
// footnote with no label at all -- and every test in this file exists to
// check that numbering.
func parseNumberingFootnotes(src string) *doctree.Element {
	opts := DefaultOptions()
	opts.NumberAutoFootnotes = true
	return ParseWithOptions(src, opts)
}

// TestFootnoteNumberingIsOptIn pins both sides, and the one part that is
// NOT optional. Numbering is transforms.references.Footnotes, so a bare
// parse leaves an auto footnote with no <label> -- but its id is assigned
// at PARSE time (document.note_autofootnote -> set_id), so that stays
// either way. Auto and symbol footnotes share one positional counter.
func TestFootnoteNumberingIsOptIn(t *testing.T) {
	const src = ".. [#] a\n\n.. [#note] b\n\n.. [*] c\n"

	byDefault := doctree.Dump(Parse(src))
	if strings.Contains(byDefault, "<label>") {
		t.Errorf("the DEFAULT parse numbered an auto footnote:\n%s", byDefault)
	}
	for _, want := range []string{`<footnote auto="1" id="footnote-1">`, `<footnote auto="1" id="note" name="note">`, `<footnote auto="*" id="footnote-2">`} {
		if !strings.Contains(byDefault, want) {
			t.Errorf("missing %s in the default parse:\n%s", want, byDefault)
		}
	}

	if optedIn := doctree.Dump(parseNumberingFootnotes(src)); !strings.Contains(optedIn, "<label>") {
		t.Errorf("NumberAutoFootnotes did not number:\n%s", optedIn)
	}
}
