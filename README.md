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
bullet lists, enumerated lists (arabic + `.` suffix only), field lists
(including a docutils-shaped body-indent quirk: a continuation line
indented less than the marker column, e.g. under `:date: 2026-08-30`,
still belongs to the field), definition lists, line blocks (flat — see
below), doctest blocks (kept verbatim, ">>>" prompts included), block
quotes, literal blocks (`::`), comments, directives (captured
structurally — name, arguments, raw content — never dispatched to
per-directive semantics: there is no directive registry), hyperlink
targets with reference resolution, footnotes (`[1]_`/`[#]_`/`[#name]_`/
`[*]_`), citations (`[CITE2002]_`), substitution definitions/references
(`|name|`, its content likewise captured structurally rather than
executed — a substitution definition is a directive invocation, most
often `replace::`), and inline markup for `**strong**`, `*emphasis*`,
`` ``literal`` ``, a bare `` `x` `` with no role (docutils' DEFAULT
role, `title_reference`), named/anonymous references both bare (`x_`,
`x__`) and backtick-quoted (`` `x`_ ``, `` `x`__ ``) including an
embedded URI or indirect-name target (`` `text <https://example.com>`_
``, `` `text <alias_>`_ ``, with `mailto:` auto-prefixing for an
embedded email address), interpreted text with a role, prefix
(`` :role:`x` ``) or suffix (`` `x`:role: ``), for docutils' built-in
GENERIC roles (`emphasis`, `strong`, `literal`, `subscript`/`sub`,
`superscript`/`sup`, `title-reference`/`title`/`t`,
`abbreviation`/`ab`, `acronym`/`ac`) — any other role name (there is no
`.. role::` registry, same philosophy as directives) falls back to a
generic `<inline role="name">` rather than docutils' error, and
backslash escapes; and SIMPLE tables (`=====`-bordered, with an
optional `-----`-underlined column-span row and a multi-line/nested-list
cell content — docutils' own SimpleTableParser docstring example is
this parser's own test fixture, verbatim).

**Not yet ported** (see the `rst`, `explicit.go`/`fieldlist.go`/
`lineblock.go`/`inline.go`/`table.go` doc comments for the exact list
and why): option lists (deferred — complex marker grammar, rare outside
man-page-style CLI docs), GRID tables (`+---+---+`-bordered — a real 2D
cell-boundary scan in 4 directions, meaningfully more work than simple
tables, which were done first), docutils' non-generic built-in roles
(`code`, `math`, `pep-reference`, `rfc-reference`, `raw`), standalone
URI/email/PEP/RFC recognition, indirect/anonymous hyperlink *targets*
(as opposed to *references*, which — see above — are supported),
inline internal targets, a substitution reference used as a hyperlink.
Title-style consistency and enumerator-sequence validation are not
enforced, and a table's column-margin violations are never detected
(only the "last column overflows its width" case is handled, since real
content relies on it). An unresolved reference or an unknown
interpreted-text role stays a plain node instead of being rewritten to
`problematic` with an error message, a leading field list isn't
promoted to a typed `<docinfo>` node, a line block with a
deeper-indented sub-line stays FLAT instead of being nested into a
sub-`<line_block>`, footnotes/citations get no auto-numbering or
auto-symbol resolution, a resolved embedded-link reference doesn't get
the extra `<target>` sibling node docutils emits alongside it (this
parser sets `refuri`/`refname` directly on the `<reference>` instead;
resolution still works the same way since it's all done by matching
names, just without that second node), and a table gets no
`<tgroup>`/`<colspec>` column-width wrapper — all transforms/emissions
real docutils applies after (or, for tgroup/colspec, as writer-facing
metadata alongside) the initial parse (verified by comparing against
`Parser().parse(src, document)` directly, before docutils' own
transform pipeline runs, not `publish_string`'s fully-transformed
output). Sphinx's `autodoc` extension (and `napoleon`, downstream of
it) is out of scope entirely: it works by importing and introspecting
live Python code, which is not portable to pure Go.

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
hand-transcribed (for footnotes/citations/substitutions, "docutils
foreign judge" means `Parser().parse(src, document)` directly rather
than `publish_string`, to see the tree before docutils' own transforms
run — see the `rst` package doc comment). Coverage as of this writing:
`doctree` 97%, `rst` 93%. `go vet ./...` and `gofmt -l .` clean.
