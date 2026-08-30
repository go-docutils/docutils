package rst

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

func TestOptionMarkerEnd(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"-f", 2},
		{"-f  desc", 2},
		{"-f desc", 7}, // single space: no 2-space boundary found, whole line is the marker
		{"-o <v1 v2>  desc", 10},
		{"", -1},
		{"  -f", -1}, // leading space: never a marker
	}
	for _, tc := range tests {
		if got := optionMarkerEnd(tc.in); got != tc.want {
			t.Errorf("optionMarkerEnd(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestSplitOptionGroup(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"-f", []string{"-f"}},
		{"-f, --file", []string{"-f", "--file"}},
		{"-o <v1, v2>", []string{"-o <v1, v2>"}}, // comma inside <> is not a split point
	}
	for _, tc := range cases {
		got := splitOptionGroup(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("splitOptionGroup(%q) = %v, want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitOptionGroup(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestParseOptionToken(t *testing.T) {
	cases := []struct {
		in   string
		want optionToken
		ok   bool
	}{
		{"-f", optionToken{Flag: "-f"}, true},
		{"-f FILE", optionToken{Flag: "-f", Arg: "FILE", Delimiter: " "}, true},
		{"--file=FILE", optionToken{Flag: "--file", Arg: "FILE", Delimiter: "="}, true},
		{"--file FILE", optionToken{Flag: "--file", Arg: "FILE", Delimiter: " "}, true},
		{"-ovalue", optionToken{Flag: "-o", Arg: "value", Delimiter: ""}, true},
		{"-o <v1 v2>", optionToken{Flag: "-o", Arg: "<v1 v2>", Delimiter: " "}, true},
		{"", optionToken{}, false},
		{"-f a b c", optionToken{}, false}, // too many tokens
	}
	for _, tc := range cases {
		got, ok := parseOptionToken(tc.in)
		if ok != tc.ok {
			t.Errorf("parseOptionToken(%q) ok = %v, want %v", tc.in, ok, tc.ok)
			continue
		}
		if ok && got != tc.want {
			t.Errorf("parseOptionToken(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestParseOptionListDump(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"short option no arg",
			"-f            Short option, no arg.\n",
			`<document>
    <option_list>
        <option_list_item>
            <option_group>
                <option>
                    <option_string>
                        -f
            <description>
                <paragraph>
                    Short option, no arg.
`,
		},
		{
			"grouped short and long",
			"-f, --file=FILE  Grouped short+long.\n",
			`<document>
    <option_list>
        <option_list_item>
            <option_group>
                <option>
                    <option_string>
                        -f
                <option>
                    <option_string>
                        --file
                    <option_argument delimiter="=">
                        FILE
            <description>
                <paragraph>
                    Grouped short+long.
`,
		},
		{
			"multiple items with a continuation line",
			"-f  First line.\n    Continuation line.\n\n-v  Second.\n",
			`<document>
    <option_list>
        <option_list_item>
            <option_group>
                <option>
                    <option_string>
                        -f
            <description>
                <paragraph>
                    First line.
                    Continuation line.
        <option_list_item>
            <option_group>
                <option>
                    <option_string>
                        -v
            <description>
                <paragraph>
                    Second.
`,
		},
		{
			"a marker with no following content at all falls back to a plain paragraph",
			"-f\n",
			`<document>
    <paragraph>
        -f
`,
		},
		{
			"a bullet marker is never mistaken for an option",
			"- bullet, not an option\n",
			`<document>
    <bullet_list bullet="-">
        <list_item>
            <paragraph>
                bullet, not an option
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := Parse(tc.source)
			got := doctree.Dump(doc)
			if strings.TrimRight(got, "\n") != strings.TrimRight(tc.want, "\n") {
				t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
}
