package doctree

import "testing"

func TestElementAttrs(t *testing.T) {
	e := NewElement("bullet_list")
	if got := e.Attr("bullet"); got != "" {
		t.Fatalf("Attr on unset key = %q, want empty", got)
	}
	e.SetAttr("bullet", "-")
	if got := e.Attr("bullet"); got != "-" {
		t.Fatalf("Attr(%q) = %q, want %q", "bullet", got, "-")
	}
	e.SetAttr("bullet", "+")
	if got := e.Attr("bullet"); got != "+" {
		t.Fatalf("Attr(%q) after overwrite = %q, want %q", "bullet", got, "+")
	}
}

func TestAppend(t *testing.T) {
	e := NewElement("paragraph")
	e.Append(&Text{Data: "hello"})
	if len(e.Children) != 1 {
		t.Fatalf("len(Children) = %d, want 1", len(e.Children))
	}
}

func TestAsText(t *testing.T) {
	cases := []struct {
		name string
		n    Node
		want string
	}{
		{"plain text", &Text{Data: "hello"}, "hello"},
		{"element with text child", NewElement(TagTitle, &Text{Data: "Title"}), "Title"},
		{
			"nested elements concatenate",
			NewElement(TagParagraph, &Text{Data: "a "}, NewElement(TagEmphasis, &Text{Data: "b"}), &Text{Data: " c"}),
			"a b c",
		},
		{"empty element", NewElement(TagParagraph), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AsText(tc.n); got != tc.want {
				t.Errorf("AsText() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNodeName(t *testing.T) {
	if (&Text{Data: "x"}).nodeName() != "#text" {
		t.Fatal("Text.nodeName() should be #text")
	}
	if NewElement("section").nodeName() != "section" {
		t.Fatal("Element.nodeName() should be the tag")
	}
}

func TestDump(t *testing.T) {
	list := NewElement(TagBulletList, NewElement(TagListItem, NewElement(TagParagraph, &Text{Data: "x"})))
	list.SetAttr("bullet", "-")
	doc := NewElement(TagDocument,
		NewElement(TagSection,
			NewElement(TagTitle, &Text{Data: "Hi"}),
			list,
		),
	)
	want := "<document>\n" +
		"    <section>\n" +
		"        <title>\n" +
		"            Hi\n" +
		"        <bullet_list bullet=\"-\">\n" +
		"            <list_item>\n" +
		"                <paragraph>\n" +
		"                    x\n"
	if got := Dump(doc); got != want {
		t.Errorf("Dump() =\n%s\nwant:\n%s", got, want)
	}
}

func TestDumpMultipleAttrsSorted(t *testing.T) {
	e := NewElement("x")
	e.SetAttr("z", "1")
	e.SetAttr("a", "2")
	want := "<x a=\"2\" z=\"1\">\n"
	if got := Dump(e); got != want {
		t.Errorf("Dump() = %q, want %q (attrs must be sorted)", got, want)
	}
}

func TestDumpMultilineText(t *testing.T) {
	e := NewElement(TagParagraph, &Text{Data: "line one\nline two"})
	want := "<paragraph>\n    line one\n    line two\n"
	if got := Dump(e); got != want {
		t.Errorf("Dump() = %q, want %q", got, want)
	}
}

// TestDumpTrailingNewlineNotABlankLine mirrors docutils' own Text.pformat
// (nodes.py, read directly), which splits via Python's str.splitlines()
// rather than str.split("\n") — a Text node whose data ends in a literal
// "\n" (routine: a multi-line paragraph's own line-joining keeps the
// source's embedded newlines, and one can land as the last character of a
// buffered run right before an inline-markup construct starting a new
// source line, e.g. "...markup:\n:emphasis:`emphasis`") must not print a
// spurious trailing blank line. Regression: found via test_interpreted.py's
// "basics" corpus cases, not assumed from the general shape of the bug.
func TestDumpTrailingNewlineNotABlankLine(t *testing.T) {
	e := NewElement(TagParagraph,
		&Text{Data: "Explicit roles for standard inline markup:\n"},
		NewElement(TagEmphasis, &Text{Data: "emphasis"}),
	)
	want := "<paragraph>\n" +
		"    Explicit roles for standard inline markup:\n" +
		"    <emphasis>\n" +
		"        emphasis\n"
	if got := Dump(e); got != want {
		t.Errorf("Dump() =\n%s\nwant:\n%s", got, want)
	}
}

func TestDumpEmptyText(t *testing.T) {
	e := NewElement(TagParagraph, &Text{Data: ""})
	want := "<paragraph>\n"
	if got := Dump(e); got != want {
		t.Errorf("Dump() = %q, want %q (an empty Text node prints no lines, like Python's \"\".splitlines())", got, want)
	}
}
