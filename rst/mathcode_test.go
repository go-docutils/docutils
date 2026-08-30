package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

func TestMathAndCodeRoles(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"math role produces a dedicated math node, not a generic inline",
			"A :math:`x^2` term.\n",
			"<document>\n    <paragraph>\n        A \n        <math>\n            x^2\n         term.\n",
		},
		{
			"code role produces a plain literal, same as a backtick literal",
			"A :code:`x = 1` snippet.\n",
			"<document>\n    <paragraph>\n        A \n        <literal>\n            x = 1\n         snippet.\n",
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
