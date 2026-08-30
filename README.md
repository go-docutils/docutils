# docutils

Pure-Go (CGO=0) reStructuredText engine — the layer both plain reST tooling
and [Sphinx](https://www.sphinx-doc.org/) build on, not a port of Sphinx
itself. See the [org capability map](https://github.com/go-docutils) and
the project memory for the full rationale behind this scope.

## Status: early v1, core grammar only

This is a from-scratch parser modeled on the reference implementation
([`docutils.parsers.rst`](https://docutils.sourceforge.io/), Python,
public domain) — its `states.py` state-machine design and pattern tables
were read as the specification, not executed or embedded. A local
docutils 0.23 install serves as a foreign judge during development: test
fixtures are cross-checked against `publish_string(..., writer_name=
'pseudoxml')` output, but nothing in this module invokes Python at
build or run time.

**Implemented**: sections (over/underlined titles, arbitrary nesting
depth via first-seen title-style ordering), paragraphs, transitions,
bullet lists, enumerated lists (arabic + `.` suffix only), block quotes,
literal blocks (`::`), comments, directives (captured structurally —
name, arguments, raw content — never dispatched to per-directive
semantics: there is no directive registry), hyperlink targets with
reference resolution, and inline markup for `**strong**`, `*emphasis*`,
`` ``literal`` ``, `` `named reference`_ `` / `` `anonymous`__ ``, and
backslash escapes.

**Not yet ported** (see the `rst` and `explicit.go` doc comments for the
exact list): field lists, option lists, definition lists, line blocks,
tables, footnotes/citations, substitution definitions, doctest blocks,
interpreted-text roles, standalone URI/email/PEP/RFC recognition,
indirect/anonymous hyperlink targets. Title-style consistency and
enumerator-sequence validation are not enforced. An unresolved reference
stays a bare node instead of being rewritten to `problematic` with an
error message, the way real docutils does. Sphinx's `autodoc` extension
(and `napoleon`, which sits downstream of it) is out of scope entirely:
it works by importing and introspecting live Python code, which is not
portable to pure Go.

```go
import (
    "github.com/go-docutils/docutils/doctree"
    "github.com/go-docutils/docutils/rst"
)

doc := rst.Parse(source)
fmt.Print(doctree.Dump(doc)) // this project's own pseudoxml-like debug format
```

## Testing

`go test ./...`. Fixtures in `rst/parser_test.go` were generated from
this parser's own output, then eyeball-verified against the docutils
foreign judge (see the package doc comment) before being frozen — not
hand-transcribed. Coverage as of this writing: `doctree` 97%, `rst` 93%.
