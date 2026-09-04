package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestDefaultRoleDirective covers docutils.parsers.rst.directives.misc.
// DefaultRole (read directly): ".. default-role:: NAME" changes what
// BARE interpreted text (no explicit role prefix, no trailing hyperlink
// underscore) resolves to for the rest of the document — read by
// referenceOrPhrase via p.defaultRole. A bare ".. default-role::" (no
// argument) resets it back to real docutils' own standard default
// (title-reference). Argument validation reuses the SAME INFO+ERROR
// pair ".. role::"'s own base-role validation already has
// (unknownRoleDiagnostics, role.go).
func TestDefaultRoleDirective(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a known built-in role becomes the new default for bare interpreted text",
			".. default-role:: subscript\n\nThis is a `subscript`.\n",
			"<document>\n    <paragraph>\n        This is a \n        <subscript>\n            subscript\n        .\n",
		},
		{
			"an unresolvable role name is an INFO+ERROR pair, the same shape \".. role::\"'s own base validation has",
			"Must define a custom role before using it.\n\n.. default-role:: custom\n",
			"<document>\n    <paragraph>\n        Must define a custom role before using it.\n    <system_message level=\"1\" line=\"3\" type=\"INFO\">\n        <paragraph>\n            No role entry for \"custom\" in module \"docutils.parsers.rst.languages.en\".\n            Trying \"custom\" as canonical role name.\n    <system_message level=\"3\" line=\"3\" type=\"ERROR\">\n        <paragraph>\n            Unknown interpreted text role \"custom\".\n        <literal_block>\n            .. default-role:: custom\n",
		},
		{
			"a bare \".. default-role::\" resets to the standard default (title-reference)",
			".. role:: custom\n.. default-role:: custom\n\nThis text uses the `default role`.\n\n.. default-role::\n\nReturned the `default role` to its standard default.\n",
			"<document>\n    <paragraph>\n        This text uses the \n        <inline class=\"custom\">\n            default role\n        .\n    <paragraph>\n        Returned the \n        <title_reference>\n            default role\n         to its standard default.\n",
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
