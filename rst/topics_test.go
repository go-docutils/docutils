package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// TestTopicsAndSidebars covers docutils.parsers.rst.directives.body's
// BasePseudoSection/Topic/Sidebar (body.py, read directly): topic's
// REQUIRED title argument vs. sidebar's OPTIONAL one (plus sidebar's
// own :subtitle:, valid only alongside a title), :class:/:name: options,
// the "content required"/"argument required" diagnostics, and the
// nesting restriction — a topic/sidebar is only valid directly inside
// <document>/<section> (topic ALSO directly inside <sidebar>); anywhere
// else (a list item, another topic, ...) is an ERROR, checked against
// the parser's own `parent` argument, which IS the same "current
// container" real docutils checks (state_machine.node). Every case
// verified against the foreign judge (Parser().parse(), the same bare,
// pre-transform tree doctree.Dump produces).
func TestTopicsAndSidebars(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a topic with a title and body",
			".. topic:: Title\n\n   Body.\n",
			"<document>\n    <topic>\n        <title>\n            Title\n        <paragraph>\n            Body.\n",
		},
		{
			":class:/:name: options",
			".. topic:: With Options\n   :class: custom\n   :name: my point\n\n   Body.\n",
			"<document>\n    <topic class=\"custom\" id=\"my-point\" name=\"my point\">\n        <title>\n            With Options\n        <paragraph>\n            Body.\n",
		},
		{
			"a topic with no title argument at all: ERROR (title is REQUIRED, unlike sidebar)",
			".. topic::\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Error in \"topic\" directive:\n            1 argument(s) required, 0 supplied.\n        <literal_block>\n            .. topic::\n",
		},
		{
			"a topic with a title but no content: ERROR",
			".. topic:: Title\n",
			"<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Content block expected for the \"topic\" directive; none found.\n        <literal_block>\n            .. topic:: Title\n",
		},
		{
			"two sibling topics at the top level, both valid",
			".. topic:: First\n\n   Body\n\n.. topic:: Second\n\n   Body.\n",
			"<document>\n    <topic>\n        <title>\n            First\n        <paragraph>\n            Body\n    <topic>\n        <title>\n            Second\n        <paragraph>\n            Body.\n",
		},
		{
			"a sidebar with a title, :subtitle:, and a nested topic (valid: topic may nest inside sidebar)",
			".. sidebar:: Title\n   :subtitle: Outer\n\n   .. topic:: Nested\n\n      Body.\n\n   More.\n\nMore.\n",
			"<document>\n    <sidebar>\n        <title>\n            Title\n        <subtitle>\n            Outer\n        <topic>\n            <title>\n                Nested\n            <paragraph>\n                Body.\n        <paragraph>\n            More.\n    <paragraph>\n        More.\n",
		},
		{
			"a topic inside a list item is NOT one of topic's valid parents: ERROR",
			"- .. topic:: In a list\n\n     Not allowed.\n",
			"<document>\n    <bullet_list bullet=\"-\">\n        <list_item>\n            <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n                <paragraph>\n                    The \"topic\" directive may not be used within topics or body elements.\n                <literal_block>\n                    .. topic:: In a list\n                    \n                       Not allowed.\n",
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
