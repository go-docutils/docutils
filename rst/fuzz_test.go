// Copyright (c) the go-docutils authors.
// SPDX-License-Identifier: BSD-3-Clause

package rst_test

import (
	"strings"
	"testing"

	"github.com/go-docutils/docutils/doctree"
	"github.com/go-docutils/docutils/html"
	"github.com/go-docutils/docutils/latex"
	"github.com/go-docutils/docutils/rst"
)

// FuzzParse points the parser at input nobody wrote, which is the only kind it
// will ever meet in a service: this package IS the untrusted-input entry point
// of its whole stack, and it had no fuzz target at all.
//
// Four properties, each of which has failed somewhere in this stack before:
//
//   - Parse does not panic, and does not return nil. A nil tree is not a
//     refusal callers can read -- the signature has no error -- so every
//     caller would dereference it.
//   - Parse is DETERMINISTIC. The parser keeps names, ids and substitution
//     definitions in maps, and Go randomises map iteration on purpose: a
//     single range over one of them that reaches the output makes the tree
//     depend on nothing the document said. Two parses of the same bytes must
//     dump identically.
//   - the WRITERS do not panic on anything Parse produced. html.Render and
//     latex.Render are reachable only through a parsed tree, so a tree shape
//     no document normally produces is exactly what a fuzzer finds for them.
//   - a second parse of the same source is still equal after a Render has run
//     over the first tree. A writer that MUTATES the tree it was handed (a
//     sort in place, an attribute written back) would otherwise corrupt a
//     caller that renders twice, or renders to two formats.
//
// Seeded with the constructs whose parsing is stateful or recursive, since
// those are where a fuzzer needs a foothold rather than a clue: a table, a
// footnote, a substitution, an include, nested sections, and a directive with
// options. The shapes that have actually crashed something in this stack are
// first.
func FuzzParse(f *testing.F) {
	seeds := []string{
		// A cycle through a definition resolved BY NAME crashed the sibling
		// converter with "fatal error: stack overflow" (go-richdoc/rst
		// v0.13.0). This parser leaves the reference in place rather than
		// inlining it, so the shape is harmless here -- and that is worth a
		// seed precisely so it stays harmless.
		".. |a| replace:: |a|\n\nsee |a| here\n",
		".. _a: b_\n.. _b: a_\n\nsee a_ here\n",
		".. |a0| replace:: x\n.. |a1| replace:: |a0| |a0|\n\nboom |a1| boom\n",
		// A note that cites itself, the other cycle this stack has met.
		".. [1] A note citing itself [1]_.\n",
		// Tables, both spellings, including a span and a malformed frame.
		"+-------+-------------------+\n|string | Python 3.2        |\n|size   +--------+----------+\n|       | 16-bit | 32-bit   |\n+-------+--------+----------+\n",
		"=====  =====\na      b\n=====  =====\n1      2\n=====  =====\n",
		// Sections, adornments and the title-underline rules.
		"Top\n===\n\nSub\n---\n\ntext\n\n.. _label:\n\nNext\n====\n",
		"first line\n----\n",
		// Explicit markup: a directive with options, a comment, a target, a
		// footnote and a citation.
		".. code:: python\n   :number-lines:\n\n    x = 1\n",
		".. include:: /nonexistent/file.rst\n",
		".. note:: content\n\n   more\n",
		"a paragraph with a reference [#]_ and a citation [cit]_\n\n.. [#] auto\n.. [cit] cited\n",
		// Inline markup, escapes, and the null separator.
		"*emphasis* **strong** ``literal`` `title` :role:`text` |sub| _`anchor` a\\ *b*\n",
		// A line block, a definition list, an option list and a doctest.
		"| one\n| two\n\nterm\n    definition\n\n-a  option\n\n>>> doctest()\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		doc := rst.Parse(src)
		if doc == nil {
			t.Fatalf("Parse returned nil for %q", clip(src))
		}
		first := doctree.Dump(doc)

		// Deterministic: same bytes, same tree. A map iteration reaching the
		// output would show up here and nowhere else.
		again := rst.Parse(src)
		if again == nil {
			t.Fatalf("the second Parse returned nil for %q", clip(src))
		}
		if second := doctree.Dump(again); second != first {
			t.Fatalf("two parses of the same source differ for %q\nfirst:\n%s\nsecond:\n%s",
				clip(src), first, second)
		}

		// The writers, on a tree only this input produces.
		htmlOut := html.Render(doc)
		latexOut := latex.Render(doc)

		// And neither writer may have MUTATED the tree it rendered.
		if after := doctree.Dump(doc); after != first {
			t.Fatalf("rendering changed the tree for %q\nbefore:\n%s\nafter:\n%s",
				clip(src), first, after)
		}
		// A writer that silently produced nothing for a non-empty document is
		// not asserted here: an empty document, a comment-only document and a
		// document of pure whitespace all legitimately render to very little.
		_, _ = htmlOut, latexOut
	})
}

func clip(s string) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}
