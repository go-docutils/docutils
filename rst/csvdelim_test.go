package rst

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// cellTexts returns the text of every <entry> paragraph in a dump, in
// order — enough to say how a csv row was split without pinning the whole
// table's shape.
func cellTexts(dump string) []string {
	var out []string
	lines := strings.Split(dump, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) != "<paragraph>" {
			continue
		}
		if i+1 < len(lines) {
			out = append(out, strings.TrimSpace(lines[i+1]))
		}
	}
	return out
}

// TestCSVTableDelimAndKeepspace covers the two csv-table options this
// package used to refuse the whole directive over. Both map straight onto
// encoding/csv's own reader — Comma and TrimLeadingSpace — which is where
// the unsupported list came from in the first place: what is left in it
// needs a configurable QUOTE or ESCAPE character, which that reader has no
// field for, or file I/O this package does not do.
//
// sphinx's own latex.rst writes a table with ":delim: ;" inside a list
// item; refusing it turned the whole table into a <directive> node holding
// its own source.
func TestCSVTableDelimAndKeepspace(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   []string
	}{
		{
			"a semicolon delimiter",
			".. csv-table::\n   :delim: ;\n\n   a; b\n   c; d\n",
			[]string{"a", "b", "c", "d"},
		},
		{
			// "space" and "tab" are words, not characters:
			// single_char_or_whitespace_or_unicode.
			"the word space",
			".. csv-table::\n   :delim: space\n\n   a b\n",
			[]string{"a", "b"},
		},
		{
			// And a Unicode code, in any of the spellings unicode_code
			// takes.
			"a Unicode code for the pipe",
			".. csv-table::\n   :delim: U+007C\n\n   a|b\n",
			[]string{"a", "b"},
		},
		{
			// CONTROL: without :keepspace: a field's leading whitespace is
			// stripped, which is the default this always had.
			"leading space is stripped by default",
			".. csv-table::\n\n   a,   b\n",
			[]string{"a", "b"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dump := doctree.Dump(Parse(tc.source))
			got := cellTexts(dump)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("Parse(%q) cells = %q, want %q\n%s", tc.source, got, tc.want, dump)
			}
			if strings.Contains(dump, "<directive") {
				t.Errorf("Parse(%q) refused the directive:\n%s", tc.source, dump)
			}
		})
	}
}

// TestCSVTableKeepspaceKeepsIt is the other half, separate because it
// asserts the opposite of the control above: with the flag, the space
// stays.
func TestCSVTableKeepspaceKeepsIt(t *testing.T) {
	const src = ".. csv-table::\n   :keepspace:\n\n   a,   b\n"
	dump := doctree.Dump(Parse(src))
	// Both halves, because the raw source inside a REFUSED directive node
	// contains "   b" as well -- a first version of this test passed
	// without the option being implemented at all.
	if strings.Contains(dump, "<directive") {
		t.Fatalf("Parse(%q) refused the directive:\n%s", src, dump)
	}
	if !strings.Contains(dump, "<entry>\n                        <paragraph>\n                               b") &&
		!strings.Contains(dump, "   b") {
		t.Errorf("Parse(%q) stripped the space :keepspace: keeps:\n%s", src, dump)
	}
}

// TestCSVTableUnusableDelimiter is the documented fallback: an argument
// that is not a character, a word or a code is not a delimiter, and the
// directive joins the ones this package does not support rather than
// inventing a diagnostic — directive option VALUE validation is not built
// here (see the README).
func TestCSVTableUnusableDelimiter(t *testing.T) {
	const src = ".. csv-table::\n   :delim: nonsense\n\n   a, b\n"
	dump := doctree.Dump(Parse(src))
	if !strings.Contains(dump, `<directive name="csv-table">`) {
		t.Errorf("Parse(%q) should fall back to the generic directive:\n%s", src, dump)
	}
}

// TestSectionTitleStateMachineLine pins the state-machine line while a
// section TITLE is inline-parsed: docutils has read the whole title
// construct — text and its closing adornment — before the Inliner runs, so
// a message that falls back to get_source_and_line reports the
// ADORNMENT's line. sphinx's latex.rst has a code role in a title whose
// "Cannot analyze code" warning was off by exactly that one line.
func TestSectionTitleStateMachineLine(t *testing.T) {
	const pre = ".. role:: ct(code)\n   :language: tex\n\n"
	re := regexp.MustCompile(`<system_message[^>]*line="(\d+)"`)
	cases := []struct {
		name   string
		source string
		want   int
	}{
		{"an underlined title", pre + "Para.\n\nThe :ct:`x` title\n~~~~~~~~~~~~~~~~~~~~\n\nBody.\n", 7},
		{"an over- and underlined title", pre + "Para.\n\n=====================\nThe :ct:`x` title\n=====================\n\nBody.\n", 8},
		{"a title at the very top", pre + "The :ct:`x` title\n~~~~~~~~~~~~~~~~~~~~\n\nBody.\n", 5},
		// CONTROL: an ordinary paragraph keeps max(first+1, last), which
		// is one PAST its single line, not the line after a construct.
		{"an ordinary paragraph", pre + "Para.\n\nThe :ct:`x` here.\n", 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dump := doctree.Dump(Parse(tc.source))
			m := re.FindStringSubmatch(dump)
			if m == nil {
				t.Fatalf("Parse(%q) raised no message:\n%s", tc.source, dump)
			}
			got, _ := strconv.Atoi(m[1])
			if got != tc.want {
				t.Errorf("Parse(%q): line = %d, want %d", tc.source, got, tc.want)
			}
		})
	}
}
