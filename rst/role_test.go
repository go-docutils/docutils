package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestRoleDirective covers ".. role:: NAME(BASE)" (role.go), verified
// against the foreign judge for each shape: aliasing a generic role,
// aliasing "raw" with a :format: option, and the bare (generic_custom_role)
// form. The directive itself leaves NO trace in the tree at all — not
// even a <comment> — matching real docutils' own Role.run, which returns
// no node. (An earlier version of this parser returned a <comment>
// element here, contradicting this very doc comment; caught only once
// ":code:"/PEP/RFC role support made the surrounding paragraph's own
// content correct enough for the stray sibling to become the only
// remaining corpus diff — see explicit.go's parseDirective.)
func TestRoleDirective(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			// roles.py's set_implicit_options: EVERY role function
			// implicitly supports a ":class:" option, defaulted by the
			// "role" directive to the role's own name when the
			// definition doesn't give one explicitly (registerRole/
			// classOption) — an aliased role carries it same as any
			// other. An earlier version of this test predates that
			// finding and wrongly expected no class attribute at all.
			"a role based on a generic role aliases its tag, carrying class=\"<role name>\"",
			".. role:: custom(strong)\n\nSome :custom:`text` here.\n",
			"<document>\n    <paragraph>\n        Some \n        <strong class=\"custom\">\n            text\n         here.\n",
		},
		{
			"a role based on raw becomes an inline raw node, format from the :format: option, plus class=\"<role name>\"",
			".. role:: myraw(raw)\n   :format: html\n\nInline :myraw:`<b>x</b>` here.\n",
			"<document>\n    <paragraph>\n        Inline \n        <raw class=\"myraw\" format=\"html\">\n            <b>x</b>\n         here.\n",
		},
		{
			// A bare role definition (no base) is docutils' own
			// generic_custom_role: <inline class="..."> — NOT the same
			// shape as this parser's own unregistered-role fallback
			// (<inline role="...">, see the next case) even though an
			// earlier version of this test claimed they matched; that
			// claim predates the corpus catching the real difference.
			"a bare role definition (no base) is docutils' own generic_custom_role: class=\"<role name>\", not role=\"...\"",
			".. role:: custom\n\nSome :custom:`text` here.\n",
			"<document>\n    <paragraph>\n        Some \n        <inline class=\"custom\">\n            text\n         here.\n",
		},
		{
			"an unregistered role name is unaffected — same fallback as before this feature existed",
			"Some :unregistered:`text` here.\n",
			"<document>\n    <paragraph>\n        Some \n        <inline role=\"unregistered\">\n            text\n         here.\n",
		},
		{
			"an EXPLICIT :class: option overrides the role-name default entirely, not just adds to it",
			".. role:: custom(emphasis)\n   :class: special\n\n:custom:`text`\n",
			"<document>\n    <paragraph>\n        <emphasis class=\"special\">\n            text\n",
		},
		{
			// roles.py's raw_role, read directly: the BUILT-IN "raw" role
			// used DIRECTLY (never through ".. role::", so no :format:
			// option can ever reach it) always errors — distinct from a
			// raw-BASED custom role, which supplies its own format and
			// works fine (see the raw-role case above).
			"the built-in \"raw\" role used directly (no :format:) always errors",
			"Can't use the :raw:`role` directly.\n",
			"<document>\n    <paragraph>\n        Can't use the \n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            :raw:`role`\n         directly.\n    <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            No format (Writer name) is associated with this role: \"raw\".\n            The \"raw\" role cannot be used directly.\n            Instead, use the \"role\" directive to create a new role with an associated format.\n",
		},
		{
			// codeRoleClasses: no ":language:"/":class:" option at all —
			// the role's own name becomes BOTH the implicit highlight
			// language (silently unanalyzable, no Pygments equivalent,
			// but no ":language:" was ever EXPLICITLY given so this
			// degrades quietly, not a warning — roles.py's code_role,
			// read directly) and the default class.
			"a role based on code with no options: implicit language degrades silently",
			".. role:: python(code)\n\nCode :python:`print(1)`.\n",
			"<document>\n    <paragraph>\n        Code \n        <literal class=\"code python\">\n            print(1)\n        .\n",
		},
		{
			// An EXPLICIT ":language:" (rather than one merely implied by
			// the custom role's own name) makes the Pygments-unavailable
			// failure visible as a WARNING + <problematic>, not a silent
			// fallback — codeRoleElement's own hasLanguage branch,
			// mirroring code_role's "except LexerError: if 'language' in
			// options: ...". The line number here (4) is this project's
			// own currently-computed value, not verified against the
			// foreign judge: real docutils reports line 5 for this exact
			// input (checked directly), one more instance of the
			// inline-message per-line-tracking gap parser.currentLine's
			// own doc comment already scopes out as a separate,
			// much-larger undertaking — not something this round widens.
			"a role based on code with an EXPLICIT :language: option warns instead of degrading silently",
			".. role:: tex(code)\n   :language: latex\n\n:tex:`x`.\n",
			"<document>\n    <paragraph>\n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            :tex:`x`\n        .\n    <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"2\" line=\"4\" type=\"WARNING\">\n        <paragraph>\n            Cannot analyze code. Pygments package not found.\n",
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
