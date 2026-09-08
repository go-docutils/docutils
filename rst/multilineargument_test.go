package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestMultiLineDirectiveArgument pins how a directive argument spanning
// two source lines is joined. parse_directive_arguments builds
// "arg_text = '\n'.join(arg_block)" and, for a directive declaring
// final_argument_whitespace, hands the whole text through with its
// internal whitespace intact -- so the LINE BREAK survives. This parser
// joined with a space, which reads the same but is not the same text.
//
// Only a multi-line argument is affected; a single-line one is identical
// either way, which is the control below.
//
// The image case is the one that goes the other way: directives.uri
// removes ALL whitespace, so a URI split across lines closes up rather
// than keeping the break.
func TestMultiLineDirectiveArgument(t *testing.T) {
	cases := []struct {
		name, source, want string
	}{
		{
			"a rubric argument keeps its line break",
			".. rubric:: This is\n   a multiline rubric\n",
			"<rubric>\n        This is\n        a multiline rubric\n",
		},
		{
			"a single-line rubric is unchanged",
			".. rubric:: One line\n",
			"<rubric>\n        One line\n",
		},
		{
			"an admonition title keeps it too",
			".. admonition:: This is\n   a long title\n\n   body\n",
			"<title>\n            This is\n            a long title\n",
		},
		{
			// The counter-case: a URI closes up instead.
			"an image URI across two lines closes up",
			".. image:: a\n   b.png\n",
			`<image uri="ab.png">`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := doctree.Dump(Parse(tc.source)); !strings.Contains(got, tc.want) {
				t.Errorf("missing %q:\n%s", tc.want, got)
			}
		})
	}
}

// TestMathRoleKeepsBackslashes pins the one roleTags entry that does NOT
// unescape its content: math_role calls unescape(text,
// restore_backslashes=True), because a backslash in math is TeX syntax
// rather than reST escaping. The math DIRECTIVE already kept its source
// verbatim for the same reason; the ROLE did not, so ":math:`A \land R`"
// lost the command that gives it meaning.
//
// The literal and emphasis cases are the control: they DO unescape, so
// this is a property of math rather than of roles.
func TestMathRoleKeepsBackslashes(t *testing.T) {
	got := doctree.Dump(Parse("x :math:`A \\land R` y\n"))
	if want := "<math>\n            A \\land R\n"; !strings.Contains(got, want) {
		t.Errorf("the math role lost its backslash:\n%s", got)
	}
	for _, role := range []string{"literal", "emphasis"} {
		got := doctree.Dump(Parse("x :" + role + ":`A \\land R` y\n"))
		if strings.Contains(got, `\land`) {
			t.Errorf("the %s role kept a backslash it should have consumed:\n%s", role, got)
		}
	}
}
