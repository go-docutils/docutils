package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

func TestDocInfoPromotion(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"registered bibliographic fields become typed docinfo children",
			":Author: Jane Doe\n:Date: 2026-08-30\n:Version: 1.0\n\nBody paragraph.\n",
			"<document>\n    <docinfo>\n        <author>\n            Jane Doe\n        <date>\n            2026-08-30\n        <version>\n            1.0\n    <paragraph>\n        Body paragraph.\n",
		},
		{
			"authors splits on ; and an unrecognized field name stays a plain field, folded into docinfo",
			":Authors: Jane Doe; John Smith\n:Custom Field: some value\n\nBody.\n",
			"<document>\n    <docinfo>\n        <authors>\n            <author>\n                Jane Doe\n            <author>\n                John Smith\n        <field>\n            <field_name>\n                Custom Field\n            <field_body>\n                <paragraph>\n                    some value\n    <paragraph>\n        Body.\n",
		},
		{
			"authors falls back to splitting on , when there is no ;",
			":Authors: Jane Doe, John Smith\n\nBody.\n",
			"<document>\n    <docinfo>\n        <authors>\n            <author>\n                Jane Doe\n            <author>\n                John Smith\n    <paragraph>\n        Body.\n",
		},
		{
			"dedication and abstract become sibling topic elements, not docinfo children",
			":Dedication: To my cat.\n\n:Abstract: This paper is about nothing.\n\nBody.\n",
			"<document>\n    <topic class=\"dedication\">\n        <title>\n            Dedication\n        <paragraph>\n            To my cat.\n    <topic class=\"abstract\">\n        <title>\n            Abstract\n        <paragraph>\n            This paper is about nothing.\n    <paragraph>\n        Body.\n",
		},
		{
			"a field list that isn't the document's own first child is left alone",
			"Body first.\n\n:key: value\n",
			"<document>\n    <paragraph>\n        Body first.\n    <field_list>\n        <field>\n            <field_name>\n                key\n            <field_body>\n                <paragraph>\n                    value\n",
		},
		{
			"a compound field body (more than one paragraph) is not promoted, matching real docutils",
			":Author: first paragraph\n\n  second paragraph\n\nBody.\n",
			"<document>\n    <docinfo>\n        <field>\n            <field_name>\n                Author\n            <field_body>\n                <paragraph>\n                    first paragraph\n                <paragraph>\n                    second paragraph\n    <paragraph>\n        Body.\n",
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
