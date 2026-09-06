package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestTestDirectiveReportsTheBlockSplit uses misc.TestDirective for what
// it is good for: it echoes back exactly how the directive block was
// split, so these cases assert parseDirectiveBlock's own contract rather
// than the directive's. Every expectation is the reference's literal
// output.
//
// The distinction the last three pin is the one that made three fixtures
// fail: whether an indented block is the ARGUMENT or the CONTENT depends
// entirely on the blank line between it and the directive marker, which
// gatherExplicitBody has already removed by the time a directive handler
// sees the body.
func TestTestDirectiveReportsTheBlockSplit(t *testing.T) {
	const d = ".. reStructuredText-test-directive"
	cases := []struct {
		name, source, want string
	}{
		{
			"nothing at all",
			d + "::\n\nParagraph.\n",
			`arguments=[], options={}, content: None`,
		},
		{
			"an optional space is allowed before the ::",
			d + " ::\n\nParagraph.\n",
			`arguments=[], options={}, content: None`,
		},
		{
			"a same-line argument",
			d + ":: argument\n\nParagraph.\n",
			`arguments=['argument'], options={}, content: None`,
		},
		{
			"an argument plus an option below it",
			d + ":: argument\n   :option: value\n\nParagraph.\n",
			`arguments=['argument'], options={'option': 'value'}, content: None`,
		},
		{
			"an option on the directive's OWN line, with no argument",
			d + ":: :option: value\n\nParagraph.\n",
			`arguments=[], options={'option': 'value'}, content: None`,
		},
		{
			"an option value spanning several lines joins with a newline",
			d + ":: argument\n   :option: * value1\n            * value2\n\nParagraph.\n",
			"arguments=['argument'], options={'option': '* value1\\n* value2'}, content: None",
		},
		{
			// No blank line: the indented block is the ARGUMENT.
			"an indented block with NO blank line before it is the argument",
			d + "::\n   Directive block contains one paragraph, no blank line before.\n\nParagraph.\n",
			`arguments=['Directive block contains one paragraph, no blank line before.'], options={}, content: None`,
		},
		{
			// One blank line: the same block is CONTENT.
			"a blank line before it makes the same block the content",
			d + "::\n\n   Directive block contains one paragraph, with a blank line before.\n\nParagraph.\n",
			`arguments=[], options={}, content:`,
		},
		{
			"two blank lines make no further difference",
			d + "::\n\n\n   Directive block contains one paragraph, with two blank lines before.\n\nParagraph.\n",
			`arguments=[], options={}, content:`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if !strings.Contains(got, tc.want) {
				t.Errorf("missing %q:\n%s", tc.want, got)
			}
			if strings.Contains(got, "Unknown directive type") {
				t.Errorf("a REGISTERED directive was reported as unknown:\n%s", got)
			}
		})
	}
}

// TestTestDirectiveContentIsVerbatim pins that the content reaches the
// <literal_block> unparsed: backslashes and inline-markup characters
// survive, because nothing interprets them.
func TestTestDirectiveContentIsVerbatim(t *testing.T) {
	got := doctree.Dump(Parse(".. reStructuredText-test-directive::\n\n   Directive \\block \\*contains* \\\\backslashes.\n"))
	if want := `Directive \block \*contains* \\backslashes.`; !strings.Contains(got, want) {
		t.Errorf("content was not verbatim, missing %q:\n%s", want, got)
	}
	if strings.Contains(got, "<emphasis>") {
		t.Errorf("the content was inline-parsed:\n%s", got)
	}
}

// TestTestDirectiveOptionValidation covers its one-entry option_spec:
// "option" is unchanged_required, so giving it no value is an error, and
// any other name is unknown. The first is the reference's own wording,
// checked against it; the second reuses target-notes' (v0.74.0).
func TestTestDirectiveOptionValidation(t *testing.T) {
	got := doctree.Dump(Parse(".. reStructuredText-test-directive:: :option:\n\nParagraph.\n"))
	for _, want := range []string{
		`Error in "reStructuredText-test-directive" directive:`,
		`invalid option value: (option: "option"; value: None)`,
		`argument required but none supplied.`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
}

// TestTestDirectiveUnindentWarning: its body ending on an unindent
// rather than a blank line draws the same warning every other explicit
// construct gets. The directive reports the split it DID make (the
// indented line is still the argument) and the warning follows.
func TestTestDirectiveUnindentWarning(t *testing.T) {
	got := doctree.Dump(Parse(".. reStructuredText-test-directive::\n   block\nno blank line.\n\nParagraph.\n"))
	if !strings.Contains(got, `arguments=['block'], options={}, content: None`) {
		t.Errorf("wrong split:\n%s", got)
	}
	if !strings.Contains(got, "Explicit markup ends without a blank line; unexpected unindent.") {
		t.Errorf("missing the unindent warning:\n%s", got)
	}
}

// TestPyReprQuoteSwitching pins Python's own rule, which pyRepr used to
// approximate with a note saying no value could reach it holding a
// quote. misc.TestDirective echoes an arbitrary argument, so one can.
func TestPyReprQuoteSwitching(t *testing.T) {
	cases := map[string]string{
		"plain":        `'plain'`,
		"it's":         `"it's"`,
		`say "hi"`:     `'say "hi"'`,
		`both ' and "`: `'both \' and "'`,
		"back\\slash":  `'back\\slash'`,
		"line\nbreak":  `'line\nbreak'`,
		"tab\there":    `'tab\there'`,
	}
	for in, want := range cases {
		if got := pyRepr(in); got != want {
			t.Errorf("pyRepr(%q) = %s, want %s", in, got, want)
		}
	}
}
