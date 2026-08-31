package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestPEPAndRFCRoles covers docutils' pep_reference_role/rfc_reference_role
// (roles.py, read directly): a valid number produces a <reference> to the
// canonical page, an out-of-range or unparseable one produces an ERROR +
// <problematic> instead. Every case verified against the foreign judge.
func TestPEPAndRFCRoles(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"PEP with a valid number becomes a reference, zero-padded to 4 digits",
			":PEP:`8`\n",
			"<document>\n    <paragraph>\n        <reference refuri=\"https://peps.python.org/pep-0008\">\n            PEP 8\n",
		},
		{
			"PEP 0 is the lowest valid number",
			":PEP:`0`\n",
			"<document>\n    <paragraph>\n        <reference refuri=\"https://peps.python.org/pep-0000\">\n            PEP 0\n",
		},
		{
			"a negative PEP number is an error",
			":PEP:`-1`\n",
			"<document>\n    <paragraph>\n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            :PEP:`-1`\n    <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            PEP number must be a number from 0 to 9999; \"-1\" is invalid.\n",
		},
		{
			"RFC with a valid number becomes a reference",
			":RFC:`2822`\n",
			"<document>\n    <paragraph>\n        <reference refuri=\"https://tools.ietf.org/html/rfc2822.html\">\n            RFC 2822\n",
		},
		{
			"RFC 0 is invalid (RFC numbers start at 1)",
			":RFC:`0`\n",
			"<document>\n    <paragraph>\n        <problematic id=\"problematic-1\" refid=\"system-message-1\">\n            :RFC:`0`\n    <system_message backref=\"problematic-1\" id=\"system-message-1\" level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            RFC number must be a number greater than or equal to 1; \"0\" is invalid.\n",
		},
		{
			"RFC with a #section suffix keeps it in the reference URI",
			":RFC:`2822#section1`\n",
			"<document>\n    <paragraph>\n        <reference refuri=\"https://tools.ietf.org/html/rfc2822.html#section1\">\n            RFC 2822\n",
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
