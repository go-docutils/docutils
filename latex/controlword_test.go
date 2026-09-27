package latex_test

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/latex"
	"github.com/go-docutils/docutils/rst"
)

// render is the whole pipeline these cases are about: reST in, LaTeX out.
func render(t *testing.T, src string) string {
	t.Helper()
	return latex.Render(rst.Parse(src))
}

// gluedControlWord reports a control word this writer emits that has run into
// the text after it -- "\noindentFirst" rather than "\noindent First". TeX
// reads a control word as a backslash plus the LONGEST run of letters, so the
// glued form is a different, undefined command and the document stops there.
// The check is written against the writer's own vocabulary of commands emitted
// WITHOUT a following brace, since those are the only ones that can glue: a
// "\emph{" is terminated by its own argument.
func gluedControlWord(tex string) string {
	for _, cmd := range []string{`\noindent`, `\par`, `\hline`, `\hrulefill`} {
		for i := 0; ; {
			j := strings.Index(tex[i:], cmd)
			if j < 0 {
				break
			}
			at := i + j + len(cmd)
			i = at
			if at >= len(tex) {
				break
			}
			c := tex[at]
			if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
				end := at
				for end < len(tex) && (tex[end] >= 'a' && tex[end] <= 'z' || tex[end] >= 'A' && tex[end] <= 'Z') {
					end++
				}
				return tex[at-len(cmd) : end]
			}
		}
	}
	return ""
}

// TestAFootnoteDoesNotSwallowItsFirstWord pins the defect a compile found and
// a content probe could not: every character of ".. [#] First" reached the
// output, in order, and the output was still a document no TeX can read,
// because "\par\noindent" ran straight into "First".
func TestAFootnoteDoesNotSwallowItsFirstWord(t *testing.T) {
	cases := []struct {
		name, src string
	}{
		{"unnamed auto footnote", ".. [#] First\n"},
		{"unnamed auto citation-style label", ".. [*] Starred\n"},
		{"named footnote", ".. [1] Numbered\n"},
		{"named citation", ".. [CIT] Cited\n"},
	}
	// Only the two unnamed cases glue at all: a named footnote gets a
	// \hypertarget next, and that backslash ends the control word by
	// accident. They are kept because the SECOND assertion still
	// discriminates for them -- the unterminated form was
	// "\noindent\hypertarget" -- but the glue check above passes for them
	// either way, so it is not what makes them a witness.
	for _, c := range cases {
		tex := render(t, c.src)
		if g := gluedControlWord(tex); g != "" {
			t.Errorf("%s: control word swallowed the next word: %q\nin:\n%s", c.name, g, tex)
		}
		if !strings.Contains(tex, `\par\noindent `) {
			t.Errorf("%s: expected a terminated \\noindent, got:\n%s", c.name, tex)
		}
	}
}

// TestNoindentAddsNoSpaceToTheText is the other half: a space used to end a
// control word is DISCARDED by TeX, so terminating \noindent must not have
// inserted a space into the footnote's own text. Checked on the bytes, since
// that is all this package controls.
func TestNoindentAddsNoSpaceToTheText(t *testing.T) {
	tex := render(t, ".. [#] First\n")
	if !strings.Contains(tex, `\noindent First`) {
		t.Fatalf("expected %q, got:\n%s", `\noindent First`, tex)
	}
	if strings.Contains(tex, `\noindent  First`) {
		t.Errorf("a second space reached the text:\n%s", tex)
	}
}

// TestMathEnvironmentIsChosenNotFixed pins pick_math_environment. Each
// expectation was taken from the reference writer run on that exact source,
// not from reading the rule -- and the matrix case is why: the rule strips
// environments before looking for a line break, so a "\\" inside
// \begin{matrix} does NOT select align*, which the obvious
// strings.Contains reading gets wrong.
func TestMathEnvironmentIsChosenNotFixed(t *testing.T) {
	cases := []struct {
		name, src, want, notWant string
	}{
		{
			name:    "top-level line break selects align*",
			src:     ".. math::\n\n   S &= \\pi r^2 \\\\\n   V &= \\frac{4}{3} \\pi r^3\n",
			want:    `\begin{align*}`,
			notWant: `\begin{equation*}`,
		},
		{
			name:    "one-liner stays equation*",
			src:     ".. math::\n\n   \\alpha = \\beta\n",
			want:    `\begin{equation*}`,
			notWant: `\begin{align*}`,
		},
		{
			name:    "a line break INSIDE an environment stays equation*",
			src:     ".. math::\n\n   \\begin{matrix} a \\\\ b \\end{matrix}\n",
			want:    `\begin{equation*}`,
			notWant: `\begin{align*}`,
		},
	}
	for _, c := range cases {
		tex := render(t, c.src)
		if !strings.Contains(tex, c.want) {
			t.Errorf("%s: want %q in:\n%s", c.name, c.want, tex)
		}
		if strings.Contains(tex, c.notWant) {
			t.Errorf("%s: did not want %q in:\n%s", c.name, c.notWant, tex)
		}
		// Whichever environment was chosen, it must CLOSE with the same name.
		if strings.Count(tex, `\begin{align*}`) != strings.Count(tex, `\end{align*}`) ||
			strings.Count(tex, `\begin{equation*}`) != strings.Count(tex, `\end{equation*}`) {
			t.Errorf("%s: environment opened and closed with different names:\n%s", c.name, tex)
		}
	}
}
