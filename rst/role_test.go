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

// TestRoleDirectiveDiagnostics covers Role.run's five distinct ERROR/INFO
// paths (read directly), none of which previously produced any diagnostic
// at all — the whole ".. role::" definition just silently vanished,
// registering nothing, the same way an ordinary malformed directive
// argument does elsewhere in this parser (a deliberate leniency for
// UNRECOGNIZED directive/role NAMES, documented in roleElement's own doc
// comment — these five cases are different: the directive name IS
// recognized, its own definition is what's malformed, which real
// docutils always surfaces).
func TestRoleDirectiveDiagnostics(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"an unresolvable base role produces an INFO plus an ERROR, not a silent registration",
			".. role:: custom(unknown-role)\n",
			"<document>\n    <system_message level=\"1\" line=\"1\" type=\"INFO\">\n        <paragraph>\n            No role entry for \"unknown-role\" in module \"docutils.parsers.rst.languages.en\".\n            Trying \"unknown-role\" as canonical role name.\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Unknown interpreted text role \"unknown-role\".\n        <literal_block>\n            .. role:: custom(unknown-role)\n",
		},
		{
			"an explicit :class: value that can't become a class name is an error naming the option",
			".. role:: custom\n   :class: 1\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Error in \"role\" directive:\n            invalid option value: (option: \"class\"; value: '1')\n            cannot make \"1\" into a class name.\n        <literal_block>\n            .. role:: custom\n               :class: 1\n",
		},
		{
			"a role NAME that can't become a class name (no :class: override) is an error naming the argument",
			".. role:: 1\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Invalid argument for \"role\" directive:\n            cannot make \"1\" into a class name.\n        <literal_block>\n            .. role:: 1\n",
		},
		{
			"an argument that doesn't match the NAME(BASE) pattern at all is an error",
			".. role:: (error)\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            \"role\" directive arguments not valid role names: \"(error)\".\n        <literal_block>\n            .. role:: (error)\n",
		},
		{
			"no content at all (neither same-line nor indented) is an error",
			".. role::\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            \"role\" directive requires arguments on the first line.\n        <literal_block>\n            .. role::\n",
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

// TestRoleDirectiveChainedBase guards a real regression risk introduced
// alongside TestRoleDirectiveDiagnostics: base-role validation must ALSO
// accept a base that's a PREVIOUSLY ".. role::"-defined custom role, not
// just knownRoleNames' static built-in set — verified directly against
// the foreign judge that real docutils' own definition-time lookup
// (roles.role's "_roles" local-registry check comes before its language-
// module fallback) accepts a chained base without any diagnostic at all.
// This does NOT assert the chain resolves all the way through at USE
// time (roleElement only ever checks ONE level of def.base against
// roleTags directly, a pre-existing, deliberately out-of-scope
// simplification documented in this file's own SCOPE note) — "widget"
// falls back to the existing generic <inline role="widget"> shape the
// same as any other base roleElement can't resolve, which is fine; what
// this guards is that DEFINING "widget" doesn't itself produce a
// spurious "Unknown interpreted text role" diagnostic it never should.
func TestRoleDirectiveChainedBase(t *testing.T) {
	src := ".. role:: gadget(strong)\n\n.. role:: widget(gadget)\n\nSee :widget:`x`.\n"
	got := doctree.Dump(Parse(src))
	if strings.Contains(got, "system_message") {
		t.Errorf("Parse(%q) dump wrongly produced a diagnostic for a chained custom-role base:\n%s", src, got)
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

// TestRoleNameGrammar pins the role name to docutils' own `simplename`
// (states.py: the role group is `(?P<role>:%(simplename)s:)?`). This used
// to scan only letters, digits and "-", so any role name containing ".",
// "_", "+" or ":" was missed entirely and left as plain text beside a bare
// <title_reference>. Every case here was checked against the reference
// implementation for RECOGNITION; the resulting node is this parser's own
// deliberate lenient <inline role="..."> rather than real docutils'
// unknown-role <problematic> (see README), which is why these assert the
// role NAME rather than a full dump.
func TestRoleNameGrammar(t *testing.T) {
	cases := []struct{ source, wantRole string }{
		{":very.long-role_name:`interpreted`\n", "very.long-role_name"},
		{":a+b:`x`\n", "a+b"},
		// scanSimpleName is greedy over ":" too, matching real docutils:
		// ":a:b:`x`" is the SINGLE role "a:b", not the role "a".
		{":a:b:`x`\n", "a:b"},
		{":a_b:`x`\n", "a_b"},
	}
	for _, tc := range cases {
		got := doctree.Dump(Parse(tc.source))
		want := `<inline role="` + tc.wantRole + `">`
		if !strings.Contains(got, want) {
			t.Errorf("Parse(%q) did not produce %s:\n%s", tc.source, want, got)
		}
	}
}

// TestRolePositionDiagnostics covers the two SYNTAX errors docutils raises
// in Inliner.interpreted_or_phrase_ref before it ever resolves the role:
// a role in both positions at once, and a role combined with a reference
// suffix. Neither depends on the role being known -- each was verified
// against the reference for :emphasis:, a perfectly ordinary built-in --
// which is why these carry no unknown-role INFO/ERROR even though this
// parser deliberately never emits those (README). Previously the suffix
// was silently dropped as leftover plain text.
func TestRolePositionDiagnostics(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a known role in both positions is a syntax error, not two roles",
			":emphasis:`x`:emphasis:\n",
			"<document>\n    <paragraph>\n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            :emphasis:`x`:emphasis:\n    <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"2\" line=\"1\" type=\"WARNING\">\n        <paragraph>\n            Multiple roles in interpreted text (both prefix and suffix present; only one allowed).\n",
		},
		{
			"an unknown role in both positions reports the SAME syntax error, with no unknown-role message",
			":role:`interpreted`:role:\n",
			"<document>\n    <paragraph>\n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            :role:`interpreted`:role:\n    <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"2\" line=\"1\" type=\"WARNING\">\n        <paragraph>\n            Multiple roles in interpreted text (both prefix and suffix present; only one allowed).\n",
		},
		{
			"a role prefix plus a reference suffix names the prefix position",
			":emphasis:`x`_\n",
			"<document>\n    <paragraph>\n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            :emphasis:`x`_\n    <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"2\" line=\"1\" type=\"WARNING\">\n        <paragraph>\n            Mismatch: both interpreted text role prefix and reference suffix.\n",
		},
		{
			"a role suffix plus a reference suffix names the suffix position",
			"`x`:emphasis:_\n",
			"<document>\n    <paragraph>\n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            `x`:emphasis:_\n    <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"2\" line=\"1\" type=\"WARNING\">\n        <paragraph>\n            Mismatch: both interpreted text role suffix and reference suffix.\n",
		},
		{
			"a role in ONE position with no reference suffix is still an ordinary role",
			":emphasis:`x`\n",
			"<document>\n    <paragraph>\n        <emphasis>\n            x\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := doctree.Dump(Parse(tc.source)); got != tc.want {
				t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
}
