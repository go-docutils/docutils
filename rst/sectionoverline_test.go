package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestSectionOverline covers overlined section titles and the diagnostics
// docutils' own 'Line' state produces for a malformed attempt (states.py,
// read directly): too-short overline/underline (a WARNING when the title
// still validates otherwise, an INFO reverting entirely to plain text when
// it doesn't), missing/mismatched underline, an incomplete title at EOF,
// two overlines with nothing between, title-style-consistency (a skipped
// level errors; reusing an established style returns to that level), and
// the enumerator/title disambiguation ("1. Numbered Title" is a title, not
// a one-item enumerated list, once its second line doesn't look like valid
// list-item content). Every case verified against the foreign judge
// (Parser().parse(), the same bare, pre-transform tree doctree.Dump
// produces).
func TestSectionOverline(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"basic overline title",
			"=====\nTitle\n=====\n\nContent.\n",
			"<document>\n    <section id=\"title\" name=\"title\">\n        <title>\n            Title\n        <paragraph>\n            Content.\n",
		},
		{
			"title inset under its overline is stripped, not rejected",
			"=====\n Title\n=====\n\nContent.\n",
			"<document>\n    <section id=\"title\" name=\"title\">\n        <title>\n            Title\n        <system_message level=\"2\" line=\"1\" type=\"WARNING\">\n            <paragraph>\n                Title overline too short.\n            <literal_block>\n                =====\n                 Title\n                =====\n        <paragraph>\n            Content.\n",
		},
		{
			"overline valid length but narrower than the title: warning, still a section",
			"====\nLonger Title\n====\n\nContent.\n",
			"<document>\n    <section id=\"longer-title\" name=\"longer title\">\n        <title>\n            Longer Title\n        <system_message level=\"2\" line=\"1\" type=\"WARNING\">\n            <paragraph>\n                Title overline too short.\n            <literal_block>\n                ====\n                Longer Title\n                ====\n        <paragraph>\n            Content.\n",
		},
		{
			"underline valid length but narrower than the title: warning, still a section",
			"Longer Title\n====\n\nContent.\n",
			"<document>\n    <section id=\"longer-title\" name=\"longer title\">\n        <title>\n            Longer Title\n        <system_message level=\"2\" line=\"2\" type=\"WARNING\">\n            <paragraph>\n                Title underline too short.\n            <literal_block>\n                Longer Title\n                ====\n        <paragraph>\n            Content.\n",
		},
		{
			"underline too short by BOTH the length and width test: not a title at all",
			"AB\n=\n\nContent.\n",
			"<document>\n    <system_message level=\"1\" line=\"2\" type=\"INFO\">\n        <paragraph>\n            Possible title underline, too short for the title.\n            Treating it as ordinary text because it's so short.\n    <paragraph>\n        AB\n        =\n    <paragraph>\n        Content.\n",
		},
		{
			"overline too short: not a title attempt at all, falls back to plain text",
			"=\nAB\n=\n\nContent.\n",
			"<document>\n    <system_message level=\"1\" line=\"1\" type=\"INFO\">\n        <paragraph>\n            Possible incomplete section title.\n            Treating the overline as ordinary text because it's so short.\n    <paragraph>\n        =\n        AB\n        =\n    <paragraph>\n        Content.\n",
		},
		{
			"overline with no matching underline at all",
			"=====\nTitle\n\nContent.\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Missing matching underline for section title overline.\n        <literal_block>\n            =====\n            Title\n    <paragraph>\n        Content.\n",
		},
		{
			"overline and title with nothing following at all (EOF)",
			"=====\nTitle\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Incomplete section title.\n        <literal_block>\n            =====\n            Title\n",
		},
		{
			"overline and underline use different characters",
			"=====\nTitle\n-----\n\nContent.\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Title overline & underline mismatch.\n        <literal_block>\n            =====\n            Title\n            -----\n    <paragraph>\n        Content.\n",
		},
		{
			"two overlines back to back with no title text between",
			"=====\n=====\n\nContent.\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Invalid section title or transition marker.\n        <literal_block>\n            =====\n            =====\n    <paragraph>\n        Content.\n",
		},
		{
			"skipping a level (jumping to a brand-new style two deep) is an error",
			"One\n===\n\nSub\n---\n\nBack\n====\n\nJump\n~~~~\n",
			"<document>\n    <section id=\"one\" name=\"one\">\n        <title>\n            One\n        <section id=\"sub\" name=\"sub\">\n            <title>\n                Sub\n    <section id=\"back\" name=\"back\">\n        <title>\n            Back\n        <system_message level=\"3\" line=\"10\" type=\"ERROR\">\n            <paragraph>\n                Inconsistent title style: skip from level 1 to 3.\n            <literal_block>\n                Jump\n                ~~~~\n            <paragraph>\n                Established title styles: = -\n",
		},
		{
			"reusing an established style returns to that level (no skip)",
			"One\n===\n\nPara one.\n\nTwo\n===\n\nPara two.\n",
			"<document>\n    <section id=\"one\" name=\"one\">\n        <title>\n            One\n        <paragraph>\n            Para one.\n    <section id=\"two\" name=\"two\">\n        <title>\n            Two\n        <paragraph>\n            Para two.\n",
		},
		{
			// A bare "::" literal-block-opener line is a uniform line too
			// (two colons, same character) — regression: an earlier version
			// of titleDiagnostic didn't exclude it, so it wrongly emitted
			// the "too short to be a title" INFO notice as a spurious extra
			// paragraph sibling in front of the actual literal block.
			"a bare :: literal-block marker is never a title attempt",
			"Sample:\n\n::\n\n   code line one\n   code line two\n",
			"<document>\n    <paragraph>\n        Sample:\n    <literal_block>\n        code line one\n        code line two\n",
		},
		{
			"a numbered title is a title, not a one-item enumerated list",
			"1. Numbered Title\n=================\n\nContent.\n",
			"<document>\n    <section id=\"numbered-title\" name=\"1. numbered title\">\n        <title>\n            1. Numbered Title\n        <paragraph>\n            Content.\n",
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
