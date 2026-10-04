// Copyright (c) the go-docutils authors.
// SPDX-License-Identifier: BSD-3-Clause

package rst

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
)

// writeTree writes files into a temp directory and returns its path. The include
// directive reads the real filesystem, so its tests need one.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func parseWithSource(t *testing.T, dir, name, src string) string {
	t.Helper()
	opts := DefaultOptions()
	opts.SourcePath = filepath.Join(dir, name)
	return doctree.Dump(ParseWithOptions(src, opts))
}

// TestIncludeSplicesItsContent pins what makes an inclusion an inclusion rather
// than a graft: the file's text is spliced into the INPUT, so a section title in it
// nests under the including document's sections exactly as if it had been typed
// there. Asked about the same two files, the reference answers the same tree.
func TestIncludeSplicesItsContent(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"part.rst": "Included Section\n----------------\n\nIncluded body with *emphasis*.\n",
	})
	got := parseWithSource(t, dir, "main.rst", "Top\n===\n\nBefore.\n\n.. include:: part.rst\n\nAfter.\n")
	for _, want := range []string{
		`<section id="included-section"`,
		"Included body with ",
		"<emphasis>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q in:\n%s", want, got)
		}
	}
	// The marker the inclusion appends to its own lines must not reach the tree.
	if strings.Contains(got, "end of inclusion") {
		t.Errorf("the end-of-inclusion marker was left in the tree:\n%s", got)
	}
	// And it must not provoke a diagnostic either.
	if strings.Contains(got, "system_message") {
		t.Errorf("the inclusion gained a diagnostic:\n%s", got)
	}
}

// TestIncludeWithoutASourcePathIsDisabled pins the default, which is a SAFETY
// default and not an omission: a caller that handed this parser a string has not
// asked it to read the filesystem. The reference gates the same directive behind its
// own file_insertion_enabled setting and words the refusal the same way.
func TestIncludeWithoutASourcePathIsDisabled(t *testing.T) {
	got := doctree.Dump(Parse(".. include:: part.rst\n"))
	if !strings.Contains(got, `"include" directive disabled.`) {
		t.Errorf("want the reference's own disabled warning in:\n%s", got)
	}
	if !strings.Contains(got, `level="2"`) {
		t.Errorf("the refusal should be a WARNING, as in the reference:\n%s", got)
	}
}

// TestIncludeVariants covers the clipping and the two block forms, each against the
// shape the reference produces for the same input.
func TestIncludeVariants(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"plain.txt":  "line one\nline two\nline three\nline four\n",
		"tabbed.txt": "a\tb\n",
		"marked.txt": "START\nkept one\nkept two\nEND\ntail\n",
	})
	cases := []struct {
		name, src string
		want      []string
		absent    []string
	}{
		{
			"literal",
			".. include:: plain.txt\n   :literal:\n",
			[]string{"<literal_block>", "line one", "line four"},
			nil,
		},
		{
			"code with a language",
			".. include:: plain.txt\n   :code: text\n",
			[]string{`<literal_block class="code text">`, "line one"},
			nil,
		},
		{
			"number-lines",
			".. include:: plain.txt\n   :code: text\n   :number-lines:\n",
			[]string{`<inline class="ln">`, "line one"},
			nil,
		},
		{
			"start-line and end-line",
			".. include:: plain.txt\n   :literal:\n   :start-line: 1\n   :end-line: 3\n",
			[]string{"line two", "line three"},
			[]string{"line one", "line four"},
		},
		{
			"start-after and end-before",
			".. include:: marked.txt\n   :literal:\n   :start-after: START\n   :end-before: END\n",
			[]string{"kept one", "kept two"},
			[]string{"tail"},
		},
		{
			// ":tab-width:" expands tabs in the included text before it becomes
			// a literal block, and a NEGATIVE width turns the expansion off --
			// the reference's own "unless tab_width is negative".
			"tab-width",
			".. include:: tabbed.txt\n   :literal:\n   :tab-width: 4\n",
			[]string{"a   b"},
			[]string{"\ta"},
		},
		{
			"number-lines starting elsewhere",
			".. include:: plain.txt\n   :code: text\n   :number-lines: 5\n",
			[]string{"5 ", "8 "},
			nil,
		},
		{
			"a class and a name",
			".. include:: plain.txt\n   :literal:\n   :class: wide\n   :name: listing\n",
			[]string{`class="wide"`, `name="listing"`},
			nil,
		},
	}
	for _, c := range cases {
		got := parseWithSource(t, dir, "main.rst", c.src)
		for _, want := range c.want {
			if !strings.Contains(got, want) {
				t.Errorf("%s: want %q in:\n%s", c.name, want, got)
			}
		}
		for _, absent := range c.absent {
			if strings.Contains(got, absent) {
				t.Errorf("%s: %q should have been clipped away:\n%s", c.name, absent, got)
			}
		}
	}
}

// TestIncludeErrors pins the three failures, each with the reference's own wording.
func TestIncludeErrors(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"marked.txt": "START\nkept\nEND\n",
	})
	cases := []struct{ name, src, want string }{
		{"a file that is not there", ".. include:: nope.txt\n",
			"Problems with \"include\" directive path:\nInputError: [Errno 2] No such file or directory:"},
		{"start-after not found", ".. include:: marked.txt\n   :literal:\n   :start-after: NOPE\n",
			"Problem with \"start-after\" option of \"include\" directive:\nText not found."},
		{"end-before not found", ".. include:: marked.txt\n   :literal:\n   :end-before: NOPE\n",
			"Problem with \"end-before\" option of \"include\" directive:\nText not found."},
		{"no argument", ".. include::\n",
			"Error in \"include\" directive:\n1 argument(s) required, 0 supplied."},
		// The "<name>" form reads from docutils' own bundled include directory
		// (isonum.txt and friends). This package ships no such files, so it
		// answers with the same error a missing file gets, naming what was asked
		// for rather than pretending to have it.
		{"the bundled-file form", ".. include:: <isonum.txt>\n",
			"Problems with \"include\" directive path:\nInputError: [Errno 2] No such file or directory: '<isonum.txt>'."},
	}
	for _, c := range cases {
		// Whitespace-flattened on both sides: a message's own newline comes
		// back INDENTED inside the dump, so a literal two-line expectation
		// would be testing the dump's layout rather than the wording.
		got := strings.Join(strings.Fields(parseWithSource(t, dir, "main.rst", c.src)), " ")
		want := strings.Join(strings.Fields(c.want), " ")
		if !strings.Contains(got, want) {
			t.Errorf("%s: want %q in:\n%s", c.name, want, got)
		}
	}
}

// TestIncludeIsCircularOnlyWhenNested is the pair that the end-of-inclusion marker
// exists for. The same file twice IN SEQUENCE is fine; a file that includes itself
// is not, and the reference's warning names the whole chain with "> " between its
// links.
func TestIncludeIsCircularOnlyWhenNested(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"part.rst": "Some body.\n",
		"loop.rst": "Loop body.\n\n.. include:: loop.rst\n",
	})

	twice := parseWithSource(t, dir, "main.rst", ".. include:: part.rst\n\n.. include:: part.rst\n")
	if strings.Contains(twice, "circular") {
		t.Errorf("the same file twice in sequence is not circular:\n%s", twice)
	}
	if n := strings.Count(twice, "Some body."); n != 2 {
		t.Errorf("want the body twice, got %d:\n%s", n, twice)
	}

	loop := parseWithSource(t, dir, "main.rst", ".. include:: loop.rst\n")
	if !strings.Contains(loop, "circular inclusion in \"include\" directive:") {
		t.Errorf("a self-including file should warn:\n%s", loop)
	}
	if !strings.Contains(loop, "> ") {
		t.Errorf("the warning should name the chain:\n%s", loop)
	}
	if !strings.Contains(loop, "Loop body.") {
		t.Errorf("the first inclusion's content should still be there:\n%s", loop)
	}
}

// TestANestedIncludeResolvesAgainstItsOwnFile pins the base path. The reference
// resolves every include against `document.current_source`, which follows the
// inclusion -- so a file in a subdirectory includes ITS neighbour, not one next to
// the top document.
func TestANestedIncludeResolvesAgainstItsOwnFile(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"sub/outer.rst": "Outer.\n\n.. include:: inner.rst\n",
		"sub/inner.rst": "Inner body.\n",
		"inner.rst":     "WRONG ONE, next to the document.\n",
	})
	got := parseWithSource(t, dir, "main.rst", ".. include:: sub/outer.rst\n")
	if !strings.Contains(got, "Inner body.") {
		t.Errorf("the nested include did not resolve against its own file:\n%s", got)
	}
	if strings.Contains(got, "WRONG ONE") {
		t.Errorf("the nested include resolved against the DOCUMENT's directory:\n%s", got)
	}
}
