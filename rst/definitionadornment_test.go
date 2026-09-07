package rst

import (
	"regexp"
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestAdornmentEndsADefinitionList pins that an adornment-shaped line is
// never a CONTINUING definition term. docutils' list ends on the
// unindent, Body dispatches the line itself, and whatever comes back
// starts a new list -- so two items that would look like one list are
// two, with a warning between them. The corpus fixture's own prose calls
// that "an acceptable limitation given that this will probably never
// happen in real life".
//
// All five shapes were run against the reference, including the control:
// two ORDINARY terms stay one list, which is what makes this a rule
// about adornments rather than about consecutive terms.
func TestAdornmentEndsADefinitionList(t *testing.T) {
	cases := []struct {
		name      string
		source    string
		wantLists int
		wantWarn  bool
		wantAlso  string
	}{
		{
			"two short-adornment terms are two lists",
			"==\n  def one.\n--\n  def two.\n", 2, true,
			"Possible incomplete section title.",
		},
		{
			"an ordinary term followed by a short adornment splits too",
			"term\n  def one.\n--\n  def two.\n", 2, true,
			"Possible incomplete section title.",
		},
		{
			"an ordinary term followed by a LONG adornment ends the list",
			// The long one is not demoted: it is an ERROR, and no second
			// list follows.
			"term\n  def one.\n----\n  def two.\n", 1, true,
			"Incomplete section title.",
		},
		{
			// The control. Without it these cases would also pass on a
			// parser that simply refused to put two items in one list.
			"two ordinary terms stay ONE list",
			"term\n  def one.\nterm2\n  def two.\n", 1, false, "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := doctree.Dump(Parse(tc.source))
			if n := strings.Count(got, "<definition_list>"); n != tc.wantLists {
				t.Errorf("got %d definition lists, want %d:\n%s", n, tc.wantLists, got)
			}
			warned := strings.Contains(got, "Definition list ends without a blank line")
			if warned != tc.wantWarn {
				t.Errorf("unindent warning = %v, want %v:\n%s", warned, tc.wantWarn, got)
			}
			if tc.wantAlso != "" && !strings.Contains(got, tc.wantAlso) {
				t.Errorf("missing %q:\n%s", tc.wantAlso, got)
			}
		})
	}
}

// TestShortAdornmentOpensADefinitionList is the other half: the FIRST
// term of a list may be an adornment, because by then the demotion has
// already happened. The rule is about continuing a list, not starting
// one -- a guard that refused adornments outright would lose this.
func TestShortAdornmentOpensADefinitionList(t *testing.T) {
	got := doctree.Dump(Parse("==\n  Not a title: a definition list item.\n"))
	if !strings.Contains(got, "<definition_list>") {
		t.Errorf("a demoted adornment did not open a definition list:\n%s", got)
	}
	// Indentation-agnostic: what matters is that "==" is the term's own
	// text, not how deep the dump nests it.
	if !regexp.MustCompile(`<term>\n\s+==\n`).MatchString(got) {
		t.Errorf("the adornment is not the term:\n%s", got)
	}
}
