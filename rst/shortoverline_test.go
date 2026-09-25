package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestShortOverlineOnlyOnFailure pins where the "so short" INFO belongs.
// A short overline is NOT wrong by itself: real docutils' Line.text and
// Line.underline reach short_overline only from inside their FAILURE
// branches, so "===\nOne\n===" is an ordinary section title. Checking
// the length FIRST -- which this parser used to do -- annotated every
// well-formed short title and refused to build it, and the corpus
// fixture that caught it is the one whose own prose describes the
// work-around this threshold comes from.
//
// Every case below was run against the reference, including the two
// LONG-overline controls: at four characters and up the same three
// failures produce a WARNING or an ERROR instead, which is what makes
// this a threshold rather than a blanket rule.
func TestShortOverlineOnlyOnFailure(t *testing.T) {
	cases := []struct {
		name        string
		source      string
		wantSection bool
		wantMessage string
	}{
		{
			"a three-character overline matching its title is an ordinary section",
			"===\nOne\n===\n\ntext\n", true, "",
		},
		{
			"two characters is fine too, when the title fits",
			"--\nHi\n--\n\ntext\n", true, "",
		},
		{
			"short AND too narrow for its title: demoted to text",
			"==\nTitle\n==\n\ntext\n", false, "Possible incomplete section title.",
		},
		{
			"short with a mismatched underline: demoted, not an ERROR",
			"===\nOne\n---\n\ntext\n", false, "Possible incomplete section title.",
		},
		{
			"short with no underline at all: demoted, not an ERROR",
			"===\nOne\ntext here\n", false, "Possible incomplete section title.",
		},
		{
			// The control that makes this a threshold: at four
			// characters the SAME shape is a real title plus a warning.
			"long and too narrow is a section WITH a warning",
			"====\nA longer title\n====\n\ntext\n", true, "Title overline too short.",
		},
		{
			// And the mismatch stays an ERROR at four characters.
			"long with a mismatched underline is still an ERROR",
			"=====\nOne\n-----\n\ntext\n", false, "Title overline & underline mismatch.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if hasSection := strings.Contains(got, "<section"); hasSection != tc.wantSection {
				t.Errorf("section built = %v, want %v:\n%s", hasSection, tc.wantSection, got)
			}
			switch tc.wantMessage {
			case "":
				if strings.Contains(got, "system_message") {
					t.Errorf("a well-formed title drew a diagnostic:\n%s", got)
				}
			default:
				if !strings.Contains(got, tc.wantMessage) {
					t.Errorf("missing %q:\n%s", tc.wantMessage, got)
				}
			}
		})
	}
}

// TestShortOverlineBubblesUp covers what docutils' short_overline does
// AFTER emitting its INFO: it calls previous_line(), handing the demoted
// line back to the Body state as ordinary text -- where it can be a
// title's OWN text, underlined by the very line that was rejected as its
// underline a moment ago. "...\n..." is therefore an INFO and a section
// titled "...", not a two-line paragraph.
//
// No corpus fixture can show this on its own: the one that exercises it
// (section_headers[32]) also rides on the duplicate-name line numbers
// docutils derives from its state machine's position, which this parser
// does not reproduce -- so the fixture stays red either way and the
// corpus total does not move. These cases are the instrument instead,
// each run against the reference.
func TestShortOverlineBubblesUp(t *testing.T) {
	cases := []struct {
		name, source, wantTitle string
	}{
		{"a short overline over a matching adornment becomes a title", "...\n...\n\nBody text.\n", "..."},
		{"two characters bubble up the same way", "--\n--\n\nBody text.\n", "--"},
		{"the second adornment need not match the first", "...\n---\n\nBody text.\n", "..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !strings.Contains(got, "<section") {
				t.Errorf("no section built:\n%s", got)
			}
			if want := "<title>\n            " + tc.wantTitle + "\n"; !strings.Contains(got, want) {
				t.Errorf("missing title %q:\n%s", tc.wantTitle, got)
			}
			if !strings.Contains(got, "Possible incomplete section title.") {
				t.Errorf("the INFO about the short overline is missing:\n%s", got)
			}
		})
	}
	// The control: with ordinary TEXT under it there is nothing to
	// underline the demoted line, so both lines stay a paragraph and only
	// the INFO is emitted. Without this the test above would also pass on
	// a parser that turned every short adornment into a section.
	got := doctree.Dump(Parse("...\nTitle\n\nBody.\n"))
	if strings.Contains(got, "<section") {
		t.Errorf("a demoted line with plain text under it built a section:\n%s", got)
	}
	if !strings.Contains(got, "Possible incomplete section title.") {
		t.Errorf("the INFO is missing from the control:\n%s", got)
	}
}

// TestAPunctuationLineUnderAnOverlineIsNeverATitle pins which of Line's
// own handlers runs for the line AFTER an overline. The dispatch is by
// PATTERN and 'underline' is tried before 'text' (states.py:
// Text.initial_transitions, inherited by SpecializedText and so by Line),
// so a uniform punctuation line always reaches Line.underline -- an
// "Invalid section title or transition marker" ERROR, or short_overline's
// own rewind when the overline is under four characters -- and the
// three-line Line.text path never even gets to look at it.
//
// This package tested "is line i+2 an underline equal to line i" first,
// which is true of three identical adornment lines. So "====\n====\n===="
// became a <section> whose TITLE was "====", with the remaining lines
// silently dropped, and "...\n...\n..." swallowed its own third line. Only
// the first and third lines being EQUAL reaches the bug, which is why
// "====\n----\nTitle" was already right -- and why it is a control here.
//
// The last case is the docutils testsuite's own
// test_section_headers.py[32], the last mismatch this package had in that
// corpus other than one artefact of the testsuite's own shared state: its
// third section returns to the DOCUMENT level, because the demoted line's
// underline style ('.') is the one already established there, and the
// nested reading kept it a grandchild. Every expectation below was
// compared against the reference; where it promotes a lone top-level
// section to the document title and this package does not (its own
// documented scope boundary), only that hoisting differs -- the messages,
// their lines and their order are identical.
func TestAPunctuationLineUnderAnOverlineIsNeverATitle(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a punctuation line under a LONG overline is an invalid marker, not a title",
			"====\n====\n====\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Invalid section title or transition marker.\n        <literal_block>\n            ====\n            ====\n    <transition>\n",
		},
		{
			"the same under a SHORT overline demotes the overline to text",
			"...\n...\n...\n",
			"<document>\n    <system_message level=\"1\" line=\"1\" type=\"INFO\">\n        <paragraph>\n            Possible incomplete section title.\n            Treating the overline as ordinary text because it's so short.\n    <section id=\"section-1\" name=\"...\">\n        <title>\n            ...\n        <paragraph>\n            ...\n",
		},
		{
			"CONTROL: a one-character second line matches underline too",
			"====\n-\nTitle\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Invalid section title or transition marker.\n        <literal_block>\n            ====\n            -\n    <paragraph>\n        Title\n",
		},
		{
			"CONTROL: the invalid pair does not consume the real title after it",
			"====\n====\nTitle\n====\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Invalid section title or transition marker.\n        <literal_block>\n            ====\n            ====\n    <section id=\"title\" name=\"title\">\n        <title>\n            Title\n        <system_message level=\"2\" line=\"4\" type=\"WARNING\">\n            <paragraph>\n                Title underline too short.\n            <literal_block>\n                Title\n                ====\n",
		},
		{
			"CONTROL: a genuine overlined title still works",
			"====\nTitle\n====\n",
			"<document>\n    <section id=\"title\" name=\"title\">\n        <title>\n            Title\n        <system_message level=\"2\" line=\"1\" type=\"WARNING\">\n            <paragraph>\n                Title overline too short.\n            <literal_block>\n                ====\n                Title\n                ====\n",
		},
		{
			"CONTROL: a DIFFERENT punctuation line was already an invalid marker",
			"====\n----\nTitle\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Invalid section title or transition marker.\n        <literal_block>\n            ====\n            ----\n    <paragraph>\n        Title\n",
		},
		{
			"the testsuite's own section_headers[32]",
			"...\n...\n\n...\n---\n\n...\n...\n...\n",
			"<document>\n    <system_message level=\"1\" line=\"1\" type=\"INFO\">\n        <paragraph>\n            Possible incomplete section title.\n            Treating the overline as ordinary text because it's so short.\n    <section dupname=\"...\" id=\"section-1\">\n        <title>\n            ...\n        <system_message level=\"1\" line=\"4\" type=\"INFO\">\n            <paragraph>\n                Possible incomplete section title.\n                Treating the overline as ordinary text because it's so short.\n        <section dupname=\"...\" id=\"section-2\">\n            <title>\n                ...\n            <system_message backref=\"section-2\" level=\"1\" line=\"5\" type=\"INFO\">\n                <paragraph>\n                    Duplicate implicit target name: \"...\".\n            <system_message level=\"1\" line=\"7\" type=\"INFO\">\n                <paragraph>\n                    Possible incomplete section title.\n                    Treating the overline as ordinary text because it's so short.\n    <section dupname=\"...\" id=\"section-3\">\n        <title>\n            ...\n        <system_message backref=\"section-3\" level=\"1\" line=\"8\" type=\"INFO\">\n            <paragraph>\n                Duplicate implicit target name: \"...\".\n        <paragraph>\n            ...\n",
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
