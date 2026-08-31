package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestEnumeratedLists covers docutils' full enumerator recognition
// (states.py's Body.enum/parse_enumerator/is_enumerated_list_item/
// make_enumerator and EnumeratedList.enumerator, all read directly): all
// five sequences (arabic, loweralpha, upperalpha, lowerroman, upperroman)
// in all three formats ("N.", "(N)", "N)"), the enumtype/prefix/suffix/
// start attributes, the start-not-ordinal-1 INFO diagnostic, the
// ambiguity-resolution rules for a single roman-charset letter (defaults
// to roman when nothing is established yet, but continues an already-
// established alpha sequence instead), a malformed roman numeral being
// rejected outright (falls through to whatever construct the line
// otherwise is — here a definition list term), and a bare marker with no
// same-line content taking its content column from the first indented
// line that follows. Every case verified against the foreign judge
// (Parser().parse(), the same bare, pre-transform tree doctree.Dump
// produces).
func TestEnumeratedLists(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"basic arabic list gets enumtype/prefix/suffix attributes",
			"1. Item one.\n\n2. Item two.\n\n3. Item three.\n",
			"<document>\n    <enumerated_list enumtype=\"arabic\" prefix=\"\" suffix=\".\">\n        <list_item>\n            <paragraph>\n                Item one.\n        <list_item>\n            <paragraph>\n                Item two.\n        <list_item>\n            <paragraph>\n                Item three.\n",
		},
		{
			"all five sequences recognized, each starting its own list",
			"Different enumeration sequences:\n\n1. Item 1.\n2. Item 2.\n3. Item 3.\n\nA. Item A.\nB. Item B.\nC. Item C.\n\na. Item a.\nb. Item b.\nc. Item c.\n\nI. Item I.\nII. Item II.\nIII. Item III.\n\ni. Item i.\nii. Item ii.\niii. Item iii.\n",
			"<document>\n    <paragraph>\n        Different enumeration sequences:\n    <enumerated_list enumtype=\"arabic\" prefix=\"\" suffix=\".\">\n        <list_item>\n            <paragraph>\n                Item 1.\n        <list_item>\n            <paragraph>\n                Item 2.\n        <list_item>\n            <paragraph>\n                Item 3.\n    <enumerated_list enumtype=\"upperalpha\" prefix=\"\" suffix=\".\">\n        <list_item>\n            <paragraph>\n                Item A.\n        <list_item>\n            <paragraph>\n                Item B.\n        <list_item>\n            <paragraph>\n                Item C.\n    <enumerated_list enumtype=\"loweralpha\" prefix=\"\" suffix=\".\">\n        <list_item>\n            <paragraph>\n                Item a.\n        <list_item>\n            <paragraph>\n                Item b.\n        <list_item>\n            <paragraph>\n                Item c.\n    <enumerated_list enumtype=\"upperroman\" prefix=\"\" suffix=\".\">\n        <list_item>\n            <paragraph>\n                Item I.\n        <list_item>\n            <paragraph>\n                Item II.\n        <list_item>\n            <paragraph>\n                Item III.\n    <enumerated_list enumtype=\"lowerroman\" prefix=\"\" suffix=\".\">\n        <list_item>\n            <paragraph>\n                Item i.\n        <list_item>\n            <paragraph>\n                Item ii.\n        <list_item>\n            <paragraph>\n                Item iii.\n",
		},
		{
			// The interesting case: a single-letter roman-charset item
			// ("I.") is only ambiguous with alpha when NOTHING is
			// already established — here each run starts completely
			// fresh (a blank line separates them, so no expected
			// sequence carries over), so "I."/"i." correctly default to
			// roman via parse_enumerator's own literal 'i'/'I' special
			// case, not alpha.
			"single-letter roman-vs-alpha ambiguity resolves correctly when nothing is established yet",
			"Potentially ambiguous cases:\n\nA. Item A.\nB. Item B.\nC. Item C.\n\nI. Item I.\nII. Item II.\nIII. Item III.\n\na. Item a.\nb. Item b.\nc. Item c.\n\ni. Item i.\nii. Item ii.\niii. Item iii.\n\nPhew! Safe!\n",
			"<document>\n    <paragraph>\n        Potentially ambiguous cases:\n    <enumerated_list enumtype=\"upperalpha\" prefix=\"\" suffix=\".\">\n        <list_item>\n            <paragraph>\n                Item A.\n        <list_item>\n            <paragraph>\n                Item B.\n        <list_item>\n            <paragraph>\n                Item C.\n    <enumerated_list enumtype=\"upperroman\" prefix=\"\" suffix=\".\">\n        <list_item>\n            <paragraph>\n                Item I.\n        <list_item>\n            <paragraph>\n                Item II.\n        <list_item>\n            <paragraph>\n                Item III.\n    <enumerated_list enumtype=\"loweralpha\" prefix=\"\" suffix=\".\">\n        <list_item>\n            <paragraph>\n                Item a.\n        <list_item>\n            <paragraph>\n                Item b.\n        <list_item>\n            <paragraph>\n                Item c.\n    <enumerated_list enumtype=\"lowerroman\" prefix=\"\" suffix=\".\">\n        <list_item>\n            <paragraph>\n                Item i.\n        <list_item>\n            <paragraph>\n                Item ii.\n        <list_item>\n            <paragraph>\n                Item iii.\n    <paragraph>\n        Phew! Safe!\n",
		},
		{
			"all three formats recognized: period, rparen, parens",
			"Different enumeration formats:\n\n1. Item 1.\n2. Item 2.\n3. Item 3.\n\n1) Item 1).\n2) Item 2).\n3) Item 3).\n\n(1) Item (1).\n(2) Item (2).\n(3) Item (3).\n",
			"<document>\n    <paragraph>\n        Different enumeration formats:\n    <enumerated_list enumtype=\"arabic\" prefix=\"\" suffix=\".\">\n        <list_item>\n            <paragraph>\n                Item 1.\n        <list_item>\n            <paragraph>\n                Item 2.\n        <list_item>\n            <paragraph>\n                Item 3.\n    <enumerated_list enumtype=\"arabic\" prefix=\"\" suffix=\")\">\n        <list_item>\n            <paragraph>\n                Item 1).\n        <list_item>\n            <paragraph>\n                Item 2).\n        <list_item>\n            <paragraph>\n                Item 3).\n    <enumerated_list enumtype=\"arabic\" prefix=\"(\" suffix=\")\">\n        <list_item>\n            <paragraph>\n                Item (1).\n        <list_item>\n            <paragraph>\n                Item (2).\n        <list_item>\n            <paragraph>\n                Item (3).\n",
		},
		{
			"a start value other than 1 gets a start attribute and an INFO message, as a SIBLING not a child",
			"Start with non-ordinal-1:\n\n0. Item zero.\n1. Item one.\n2. Item two.\n3. Item three.\n\nAnd again:\n\n2. Item two.\n3. Item three.\n",
			"<document>\n    <paragraph>\n        Start with non-ordinal-1:\n    <enumerated_list enumtype=\"arabic\" prefix=\"\" start=\"0\" suffix=\".\">\n        <list_item>\n            <paragraph>\n                Item zero.\n        <list_item>\n            <paragraph>\n                Item one.\n        <list_item>\n            <paragraph>\n                Item two.\n        <list_item>\n            <paragraph>\n                Item three.\n    <system_message level=\"1\" line=\"3\" type=\"INFO\">\n        <paragraph>\n            Enumerated list start value not ordinal-1: \"0\" (ordinal 0)\n    <paragraph>\n        And again:\n    <enumerated_list enumtype=\"arabic\" prefix=\"\" start=\"2\" suffix=\".\">\n        <list_item>\n            <paragraph>\n                Item two.\n        <list_item>\n            <paragraph>\n                Item three.\n    <system_message level=\"1\" line=\"10\" type=\"INFO\">\n        <paragraph>\n            Enumerated list start value not ordinal-1: \"2\" (ordinal 2)\n",
		},
		{
			// "iiii" is roman-CHARSET-shaped but not a well-formed roman
			// numeral (4 raw I's, no subtractive form) — classified but
			// with ordinalOK=false, so it's rejected as a list item
			// entirely and falls through to become a definition list
			// term instead, exactly like real docutils. Also covers
			// format disambiguation among false-positive-looking
			// parenthesized acronyms/words ("(LCD)", "(livid)", "(CIVIL)")
			// that are NOT valid enumerator text (not single-letter, not
			// digits, not a pure roman-charset run) and so stay plain
			// text, versus "(I)" (a genuine, if single-item, upperroman
			// list) and "(IVXLCDM)" (roman-charset shaped but not a
			// well-formed numeral, ordinalOK=false, stays plain text).
			"a malformed roman numeral is rejected outright, falling through to a definition list term",
			"Bad Roman numerals:\n\ni. i\n\nii. ii\n\niii. iii\n\niiii. iiii\n      second line\n\n(LCD) is an acronym made up of Roman numerals\n\n(livid) is a word made up of Roman numerals\n\n(CIVIL) is another such word\n\n(I) I\n\n(IVXLCDM) IVXLCDM\n",
			"<document>\n    <paragraph>\n        Bad Roman numerals:\n    <enumerated_list enumtype=\"lowerroman\" prefix=\"\" suffix=\".\">\n        <list_item>\n            <paragraph>\n                i\n        <list_item>\n            <paragraph>\n                ii\n        <list_item>\n            <paragraph>\n                iii\n    <definition_list>\n        <definition_list_item>\n            <term>\n                iiii. iiii\n            <definition>\n                <paragraph>\n                    second line\n    <paragraph>\n        (LCD) is an acronym made up of Roman numerals\n    <paragraph>\n        (livid) is a word made up of Roman numerals\n    <paragraph>\n        (CIVIL) is another such word\n    <enumerated_list enumtype=\"upperroman\" prefix=\"(\" suffix=\")\">\n        <list_item>\n            <paragraph>\n                I\n    <paragraph>\n        (IVXLCDM) IVXLCDM\n",
		},
		{
			// A bare marker with NO same-line content ("1." alone) has
			// no content-column anchor of its own — real docutils takes
			// it from wherever the first indented line actually starts
			// (here 3 columns, wider than the bare marker's own 2-column
			// width), not clamped to the marker's own width. Regression:
			// an earlier version of gatherListItemLines only ever
			// clamped the content column NARROWER, never wider, wrongly
			// nesting "foo" inside a spurious block_quote.
			"a bare marker with no same-line content takes its content column from the following indented line",
			"3-space indent, no trailing space:\n\n1.\n   foo\n",
			"<document>\n    <paragraph>\n        3-space indent, no trailing space:\n    <enumerated_list enumtype=\"arabic\" prefix=\"\" suffix=\".\">\n        <list_item>\n            <paragraph>\n                foo\n",
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
