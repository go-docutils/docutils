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
