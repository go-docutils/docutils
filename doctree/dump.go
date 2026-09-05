package doctree

import (
	"fmt"
	"sort"
	"strings"
)

// Dump renders the tree in an indented, pseudoxml-like form for tests
// and debugging. It is this project's own canonical debug format, not a
// byte-for-byte reproduction of docutils' pseudoxml writer (which also
// carries ids/names/source/line attributes this parser does not yet
// compute).
func Dump(n Node) string {
	var b strings.Builder
	dump(&b, n, 0)
	return b.String()
}

func dump(b *strings.Builder, n Node, depth int) {
	indent := strings.Repeat("    ", depth)
	switch v := n.(type) {
	case *Text:
		// docutils' own Text.pformat (nodes.py, read directly) splits via
		// Python's str.splitlines(), not str.split("\n") — for data ending
		// in a literal newline (routine: a paragraph's own line-joining
		// keeps embedded "\n" between source lines, and that newline can
		// land as the LAST character of a buffered run right before an
		// inline-markup construct that starts a new source line),
		// splitlines() drops the trailing empty element that split("\n")
		// keeps. Using split("\n") unconditionally printed a spurious
		// blank line in exactly that position — found via test_interpreted
		// .py's "basics" cases, not assumed from the general shape of the
		// bug.
		lines := strings.Split(v.Data, "\n")
		if last := len(lines) - 1; last >= 0 && lines[last] == "" {
			lines = lines[:last]
		}
		for _, line := range lines {
			b.WriteString(indent)
			b.WriteString(line)
			b.WriteByte('\n')
		}
	case *Element:
		b.WriteString(indent)
		b.WriteByte('<')
		b.WriteString(v.Tag)
		for _, k := range sortedKeys(v.Attrs) {
			fmt.Fprintf(b, " %s=%s", k, quoteAttr(v.Attrs[k]))
		}
		b.WriteString(">\n")
		for _, c := range v.Children {
			dump(b, c, depth+1)
		}
	}
}

// quoteAttr wraps s in double quotes. Real docutils' pseudoxml quoting
// is nodes.pseudo_quoteattr, which is literally `'"%s"' % value` — it
// escapes NOTHING, not the double-quote character and not the backslash,
// producing technically-invalid XML for a value containing a quote
// (`names=""target2""`) and not caring, because pseudoxml is a debug
// format. So neither does this: escaping the quote made an inline
// target named `"target2"` differ from the reference for no reason
// (v0.55.1). Newline and tab are the one deliberate exception — this
// dump is LINE-oriented, and a raw newline inside an attribute value
// would corrupt the shape of the output rather than merely differ from
// it. No corpus fixture exercises either, so nothing is being papered
// over here.
func quoteAttr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
