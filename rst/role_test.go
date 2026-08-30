package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestRoleDirective covers ".. role:: NAME(BASE)" (role.go), verified
// against the foreign judge for each shape: aliasing a generic role,
// aliasing "raw" with a :format: option, and the bare (generic_custom_role)
// form. The directive itself leaves no trace in the tree — a <comment>,
// same as real docutils' Role.run, which returns no node at all.
func TestRoleDirective(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a role based on a generic role aliases its tag",
			".. role:: custom(strong)\n\nSome :custom:`text` here.\n",
			"<document>\n    <comment>\n    <paragraph>\n        Some \n        <strong>\n            text\n         here.\n",
		},
		{
			"a role based on raw becomes an inline raw node, format from the :format: option",
			".. role:: myraw(raw)\n   :format: html\n\nInline :myraw:`<b>x</b>` here.\n",
			"<document>\n    <comment>\n    <paragraph>\n        Inline \n        <raw format=\"html\">\n            <b>x</b>\n         here.\n",
		},
		{
			"a bare role definition (no base) is docutils' own generic_custom_role, same as this parser's existing unregistered-role fallback",
			".. role:: custom\n\nSome :custom:`text` here.\n",
			"<document>\n    <comment>\n    <paragraph>\n        Some \n        <inline role=\"custom\">\n            text\n         here.\n",
		},
		{
			"an unregistered role name is unaffected — same fallback as before this feature existed",
			"Some :unregistered:`text` here.\n",
			"<document>\n    <paragraph>\n        Some \n        <inline role=\"unregistered\">\n            text\n         here.\n",
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

// TestInlineRawRolePreservesBackslashes guards a real bug: the shared
// interpreted-text content path unescapes backslashes (reST's own escape
// syntax, correct for every OTHER role), which would have silently eaten
// a leading "\" from raw target-format content — "\textbf{bold}" coming
// out as "textbf{bold}", breaking whatever LaTeX/HTML it was supposed to
// be. Caught by actually compiling the LaTeX case through go-tex/engine
// and reading the resulting PDF's text back, not just by diffing trees.
func TestInlineRawRolePreservesBackslashes(t *testing.T) {
	src := ".. role:: mytex(raw)\n   :format: latex\n\nInline :mytex:`\\textbf{bold}` text.\n"
	got := doctree.Dump(Parse(src))
	if !strings.Contains(got, `\textbf{bold}`) {
		t.Errorf("backslash was lost from raw role content:\n%s", got)
	}
}

func TestRawRoleDisabledFallsBackToGeneric(t *testing.T) {
	src := ".. role:: myraw(raw)\n   :format: html\n\nInline :myraw:`<b>x</b>` here.\n"
	got := doctree.Dump(ParseWithOptions(src, Options{RawEnabled: false}))
	if strings.Contains(got, "<raw") {
		t.Errorf("a raw-based role produced <raw> despite RawEnabled=false:\n%s", got)
	}
	if !strings.Contains(got, `<inline role="myraw">`) {
		t.Errorf("disabled raw role did not fall back to the generic inline shape:\n%s", got)
	}
}
