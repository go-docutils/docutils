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
		for _, line := range strings.Split(v.Data, "\n") {
			b.WriteString(indent)
			b.WriteString(line)
			b.WriteByte('\n')
		}
	case *Element:
		b.WriteString(indent)
		b.WriteByte('<')
		b.WriteString(v.Tag)
		for _, k := range sortedKeys(v.Attrs) {
			fmt.Fprintf(b, " %s=%q", k, v.Attrs[k])
		}
		b.WriteString(">\n")
		for _, c := range v.Children {
			dump(b, c, depth+1)
		}
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
