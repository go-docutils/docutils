package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

func TestMatchDirectiveName(t *testing.T) {
	cases := []struct {
		rest     string
		wantName string
		wantArgs string
		wantOK   bool
	}{
		{"note:: content", "note", "content", true},
		{"code-block:: go", "code-block", "go", true},
		{"figure::", "figure", "", true},
		{"not a directive line", "", "", false},
		{"_target: no colon-colon here", "", "", false},
		{"a comment with :: inside but no leading name-colon-colon match at start", "", "", false},
	}
	for _, tc := range cases {
		name, args, ok := matchDirectiveName(tc.rest)
		if ok != tc.wantOK || name != tc.wantName || args != tc.wantArgs {
			t.Errorf("matchDirectiveName(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.rest, name, args, ok, tc.wantName, tc.wantArgs, tc.wantOK)
		}
	}
}

func TestNormalizeName(t *testing.T) {
	cases := map[string]string{
		"Example":       "example",
		"  Some  Name ": "some name",
		"already-norm":  "already-norm",
	}
	for in, want := range cases {
		if got := normalizeName(in); got != want {
			t.Errorf("normalizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchBracketLabel(t *testing.T) {
	cases := []struct {
		s        string
		wantLbl  string
		wantRest string
		wantOK   bool
	}{
		{"[1] content", "1", "content", true},
		{"[#]", "#", "", true},
		{"[#note] content", "#note", "content", true},
		{"[*] content", "*", "content", true},
		{"[CIT2002] content", "CIT2002", "content", true},
		{"not bracketed", "", "", false},
		{"[unterminated", "", "", false},
		{"[1]nospace", "", "", false},

		// The label must be a footnote or citation label, not arbitrary text.
		// docutils dispatches on `[0-9]+|#|#simplename|\*` (footnote) and
		// `simplename` (citation); anything else falls through to the comment
		// fallback, so matchBracketLabel must refuse it rather than build a
		// <citation> out of a sentence that merely starts with a bracket.
		{"[citation label with spaces] text", "", "", false},
		{"[] empty", "", "", false},
		{"[not a name!] text", "", "", false},
		{"[_leading] text", "", "", false},
		{"[trailing-] text", "", "", false},
		{"[double--dash] text", "", "", false},
		{"[#not a name] text", "", "", false},
		{"[#] ok", "#", "ok", true},
		{"[a.b-c_d+e:f] ok", "a.b-c_d+e:f", "ok", true},
	}
	for _, tc := range cases {
		label, rest, ok := matchBracketLabel(tc.s)
		if ok != tc.wantOK || label != tc.wantLbl || rest != tc.wantRest {
			t.Errorf("matchBracketLabel(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.s, label, rest, ok, tc.wantLbl, tc.wantRest, tc.wantOK)
		}
	}
}

func TestMatchPipeLabel(t *testing.T) {
	cases := []struct {
		s        string
		wantName string
		wantRest string
		wantOK   bool
	}{
		{"|name| replace:: text", "name", "replace:: text", true},
		{"|name|", "name", "", true},
		{"not piped", "", "", false},
		{"|unterminated", "", "", false},

		// docutils brackets the name with "(?![ ])" and "(?<![ ])", so
		// whitespace immediately INSIDE either pipe disqualifies the whole
		// construct and it falls through to the comment fallback. Spaces
		// WITHIN the name are fine.
		{"| leading| x", "", "", false},
		{"|trailing | x", "", "", false},
		{"| both | x", "", "", false},
		{"|good name| replace:: x", "good name", "replace:: x", true},
	}
	for _, tc := range cases {
		name, rest, ok := matchPipeLabel(tc.s)
		if ok != tc.wantOK || name != tc.wantName || rest != tc.wantRest {
			t.Errorf("matchPipeLabel(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.s, name, rest, ok, tc.wantName, tc.wantRest, tc.wantOK)
		}
	}
}

// TestMatchPipeLabelMultiline covers the "|name|" marker's own NAME
// spanning several physical lines — real docutils progressively
// re-matches its own substitution pattern against the marker's growing
// text until the closing "|" is found (Body.substitution_def, read
// directly). The single-line cases mirror TestMatchPipeLabel's own
// (matchPipeLabelMultiline's fast path is just matchPipeLabel itself).
func TestMatchPipeLabelMultiline(t *testing.T) {
	cases := []struct {
		name             string
		lines            []string
		i                int
		wantName         string
		wantRest         string
		wantBodyStartIdx int
		wantOK           bool
	}{
		{
			"closes on the first line: bodyStartIdx unchanged",
			[]string{"|name| replace:: text"},
			0, "name", "replace:: text", 0, true,
		},
		{
			"not a pipe marker at all",
			[]string{"not piped"},
			0, "", "", 0, false,
		},
		{
			"name spans two lines, bodyStartIdx advances to the closing line",
			[]string{"|very long substitution text,", "   split across lines| image:: symbol.png"},
			0, "very long substitution text, split across lines", "image:: symbol.png", 1, true,
		},
		{
			"stops at a blank line without finding a close: malformed",
			[]string{"|unterminated", "", "   more text"},
			0, "", "", 0, false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name, rest, bodyStartIdx, ok := matchPipeLabelMultiline(tc.lines, tc.i, tc.lines[tc.i])
			if ok != tc.wantOK || name != tc.wantName || rest != tc.wantRest || (ok && bodyStartIdx != tc.wantBodyStartIdx) {
				t.Errorf("matchPipeLabelMultiline(%v, %d, ...) = (%q, %q, %d, %v), want (%q, %q, %d, %v)",
					tc.lines, tc.i, name, rest, bodyStartIdx, ok, tc.wantName, tc.wantRest, tc.wantBodyStartIdx, tc.wantOK)
			}
		})
	}
}

func TestIsAllDigits(t *testing.T) {
	cases := map[string]bool{
		"123": true,
		"":    false,
		"12a": false,
		"0":   true,
	}
	for in, want := range cases {
		if got := isAllDigits(in); got != want {
			t.Errorf("isAllDigits(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestResolveTargetsForwardReference exercises resolveTargets' two-pass
// design (collect every target first, then link references) against a
// target defined AFTER the reference that points to it — the common
// case (footnote-style targets at the end of a document).
func TestResolveTargetsForwardReference(t *testing.T) {
	doc := Parse("See `Example`_ now.\n\n.. _Example: https://example.com\n")
	got := doctree.Dump(doc)
	if !strings.Contains(got, `refuri="https://example.com"`) {
		t.Errorf("forward-referenced target was not resolved:\n%s", got)
	}
}

// TestTargetNameTerminator pins where a hyperlink target's NAME ends: at
// the first colon FOLLOWED BY whitespace or the end of the line -- not at
// the first colon of any kind, and not the last. A reference name may
// itself contain colons and so may a URI, so both simpler rules are
// wrong, in opposite directions. Every case checked against the
// reference.
func TestTargetNameTerminator(t *testing.T) {
	cases := []struct{ source, wantName, wantURI string }{
		// The name holds a colon; the trailing one terminates it, and
		// there is no URI at all -- so no refuri attribute either.
		{".. _figure:caption:\n\nx\n", "figure:caption", ""},
		// "first colon" would stop at "http", "last colon" inside the URI.
		{".. _name: http://example.org\n\nx\n", "name", "http://example.org"},
		// Both at once.
		{".. _a:b:c: http://x\n\ny\n", "a:b:c", "http://x"},
		{".. _plain:\n\nz\n", "plain", ""},
	}
	for _, tc := range cases {
		got := doctree.Dump(Parse(tc.source))
		if !strings.Contains(got, `name="`+tc.wantName+`"`) {
			t.Errorf("Parse(%q) did not name the target %q:\n%s", tc.source, tc.wantName, got)
		}
		switch tc.wantURI {
		case "":
			if strings.Contains(got, "refuri=") {
				t.Errorf("Parse(%q) gave a URI-less target a refuri:\n%s", tc.source, got)
			}
		default:
			if !strings.Contains(got, `refuri="`+tc.wantURI+`"`) {
				t.Errorf("Parse(%q) did not set refuri %q:\n%s", tc.source, tc.wantURI, got)
			}
		}
	}
}

// TestUnimplementedRoleIsNotUnknown covers the one role name that IS in
// docutils' registry purely so its function can raise a DIFFERENT error:
// roles.unimplemented_role. The name is not unknown, it is unimplemented,
// and the message says so.
func TestUnimplementedRoleIsNotUnknown(t *testing.T) {
	got := doctree.Dump(Parse(":restructuredtext-unimplemented-role:`interpreted`\n"))
	if !strings.Contains(got, `Interpreted text role "restructuredtext-unimplemented-role" not implemented.`) {
		t.Errorf("expected the not-implemented message:\n%s", got)
	}
	if strings.Contains(got, "Unknown interpreted text role") {
		t.Errorf("a REGISTERED role was reported as unknown:\n%s", got)
	}
}
