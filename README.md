# docutils

Pure-Go (CGO=0) reStructuredText engine — the layer both plain reST tooling
and [Sphinx](https://www.sphinx-doc.org/) build on, not a port of Sphinx
itself. See the [org capability map](https://github.com/go-docutils) and
the project memory for the full rationale behind this scope.

## Status: early v1, core grammar + a first writer

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
still belongs to the field) — with a leading field list (the document's
very own first child) promoted to a typed `<docinfo>`, registered
bibliographic names (author, authors, organization, address, contact,
version, revision, status, date, copyright, dedication, abstract,
matched the same case/whitespace-insensitive way as any other reST
name) becoming typed children rather than staying generic `<field>`s;
`authors` splits a single field body on `;` (or, failing that, `,`)
into one `<author>` per name; dedication/abstract become sibling
`<topic>` elements instead, right after docinfo — definition lists, line blocks (nested by
relative indentation, matching docutils' own sub-stanza grouping),
doctest blocks (kept verbatim, ">>>" prompts included), block
quotes, literal blocks (`::`), comments, directives (captured
structurally — name, arguments, raw content — never dispatched to
per-directive semantics: there is no directive registry), hyperlink
targets with reference resolution — including INDIRECT targets
(`.. _a: b_`, whose value is itself another target's name, chased
through however many hops until a real URI is reached; a cycle is left
unresolved rather than looping forever) and ANONYMOUS targets/references
(`.. __: uri` / `` x__ `` / `` `x`__ ``, matched by DOCUMENT-ORDER
POSITION — the Nth anonymous reference to the Nth anonymous target,
textual order, regardless of which comes first — rather than by name at
all, a genuinely different mechanism from every other target/reference
pair above; an anonymous target's own value may itself be indirect,
`.. __: othername_`, chased the same way a named indirect target is) —
footnotes (`[1]_`/`[#]_`/`[#name]_`/
`[*]_`, with real auto-NUMBERING (`[#]_`/`[#name]_` sharing one
sequence, an explicit `[1]_` elsewhere making the sequence skip that
number) and auto-SYMBOL assignment (`[*]_`, docutils' own fixed
ten-symbol sequence, doubling/tripling/... once it wraps: `**`, `††`,
...) — both matched to their references by document-order position
when unnamed, same mechanism as anonymous targets above), citations
(`[CITE2002]_`, never auto-numbered), substitution definitions/references
(`|name|`, its content likewise captured structurally rather than
executed — a substitution definition is a directive invocation, most
often `replace::`; `|name|_`/`|name|__` used AS a hyperlink resolves the
same way a bare/anonymous reference does, just wrapping the substitution
instead of carrying its own display text), and inline markup for `**strong**`, `*emphasis*`,
`` ``literal`` ``, a bare `` `x` `` with no role (docutils' DEFAULT
role, `title_reference`), named/anonymous references both bare (`x_`,
`x__`) and backtick-quoted (`` `x`_ ``, `` `x`__ ``) including an
embedded URI or indirect-name target (`` `text <https://example.com>`_
``, `` `text <alias_>`_ ``, with `mailto:` auto-prefixing for an
embedded email address), interpreted text with a role, prefix
(`` :role:`x` ``) or suffix (`` `x`:role: ``), for docutils' built-in
GENERIC roles (`emphasis`, `strong`, `literal`, `subscript`/`sub`,
`superscript`/`sup`, `title-reference`/`title`/`t`,
`abbreviation`/`ab`, `acronym`/`ac`), plus its other two always-registered
roles, `code` (no syntax highlighting — this parser has no
role-option syntax to carry a `:language:`, so it degrades to exactly
the plain `<literal>` real docutils itself falls back to with no
language set) and `math` (a dedicated `<math>` node holding the raw,
unescaped TeX source, rendered by both writers below) — any other role name (there is no
`.. role::` registry, same philosophy as directives) falls back to a
generic `<inline role="name">` rather than docutils' error, and
backslash escapes; standalone URI (`scheme://...`) and email
(`user@host`) recognition — no backtick quoting or trailing `_` needed
at all, e.g. plain `https://example.com` in running text becomes a
reference on its own, trailing sentence punctuation (`, and .`)
correctly excluded from the link; SIMPLE tables (`=====`-bordered, with
an optional `-----`-underlined column-span row and a
multi-line/nested-list cell content — docutils' own SimpleTableParser
docstring example is this parser's own test fixture, verbatim); and
GRID tables (`+---+---+`-bordered, `|`-separated columns, an optional
`+===+===+` head/body separator, cells spanning multiple ROWS as well
as columns — likewise docutils' own GridTableParser docstring example,
verbatim, traced with the same BFS cell-rectangle algorithm as
upstream: a queue of corner candidates, scanning right/down/left/up
around each cell to close its rectangle and discover the next cells'
starting corners); and OPTION lists (`-f, --file=ARG  Description.`,
man-page-style, comma-separated short/long flag groups with an
argument joined by a space/`=`/embedded delimiter, reusing the same
marker+indented-continuation machinery as field lists — a marker with
no following content at all, on its own line or indented beneath it,
is not really an option list item and falls back to plain paragraph
text, matching docutils' own TransitionCorrection); and inline
internal targets (`` _`text` `` — a target INSIDE a paragraph, as
opposed to the block-level `.. _name: uri` hyperlink target above,
docutils' own target pattern in `Inliner.patterns`). Unlike a
block-level target, this one keeps its content as visible text and
carries no URI of its own — a reference resolving to one now resolves
to a same-document anchor (`#name`) instead, which both writers render
with a real anchor point (HTML `<a id="name">text</a>`; LaTeX
`\hypertarget`/`\hyperlink`, not `\href` — a `#`-prefixed refuri routes
to the internal-link path specifically, since `\href`'s usual URL
escaping would corrupt hyperref's own `#`-marker convention).

**Not yet ported** (see the `rst`, `explicit.go`/`fieldlist.go`/
`lineblock.go`/`inline.go`/`table.go`/`gridtable.go` doc comments for
the exact list and why): docutils' two remaining non-generic
built-in roles, `pep-reference`/`rfc-reference` (checked against
`Parser().parse()` with default settings — docutils' own
`pep_references`/`rfc_references` settings default to **off**, so
implementing them unconditionally would diverge from upstream's own
default rather than fill a real gap) and `raw` (arbitrary raw
passthrough by format, a real security consideration for untrusted
input this parser has never had to reason about). Title-style
consistency and enumerator-sequence validation are not
enforced, and a table's column-margin violations are never detected
(only the "last column overflows its width" case is handled, since real
content relies on it). A dangling NAMED reference (bare, backtick-quoted,
or an embedded indirect alias — matched by name and found nowhere) IS
rewritten to `<problematic>`, with every such message collected into a
trailing `<section class="system-messages">`, docutils' own
DanglingReferences + Messages transforms, simplified: no
duplicate/ambiguous-name diagnostics, and `<problematic>`'s content is the
reference's own visible text rather than real docutils' verbatim source
slice (this parser doesn't track original source text on a node at all).
An ANONYMOUS reference/target count mismatch IS covered too — real
docutils checks this as a single whole-document condition, not
per-reference (`AnonymousHyperlinks.apply`, read directly): if the counts
don't match EXACTLY, in either direction, every anonymous reference in the
document becomes `<problematic>`, all sharing ONE message. An unknown
interpreted-text role still stays a plain node instead — a real docutils
error too, but this parser has no role registry at all (every unrecognized
name already looks "unknown" to it, even a real custom role a document
defines via `.. role::`), so rewriting on that basis would produce false
positives rather than fill a real gap; a role registry would need to come
first. A resolved
embedded-link reference doesn't get the extra `<target>` sibling node
docutils emits alongside it (this parser sets `refuri`/`refname` directly
on the `<reference>` instead; resolution still works the same way since
it's all done by matching names, just without that second node) —
genuinely unported transforms, unlike target/anonymous resolution,
footnote numbering, docinfo promotion, and dangling-named-reference
rewriting above, each a deliberately simplified PORT of the
corresponding real transform, not a parser-level gap (verified by comparing
against `Parser().parse(src, document)` directly, before docutils' own
transform pipeline runs, not `publish_string`'s fully-transformed
output). A table's `<tgroup cols="N">`/`<colspec colwidth="W">` wrapper
IS produced (verified: it's part of the bare parse, not a later
transform or writer-side addition, unlike the above), the `html`/
`latex` writers and `go-richdoc/rst` all unwrap it transparently.
Sphinx's `autodoc` extension (and `napoleon`, downstream of
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

## Writers

**`html`**: `html.Render(doc) string` renders a doctree to an HTML
**fragment** — body content only, no `<!DOCTYPE>`/`<html>`/`<head>`, no
stylesheet, no CSS classes or ids beyond the few this parser can
actually populate (a footnote/citation's own id, a role's name as a
`class`). This is a deliberate, bounded v1: docutils' own HTML writer
(`writers/_html_base.py` + `html5_polyglot/__init__.py`, ~2300 lines)
embeds a full default CSS stylesheet and a CSS-class vocabulary Sphinx
themes build on — replicating that byte-for-byte would be roughly
another parser's worth of work, for a stylesheet Sphinx doesn't even
use (it has its own Jinja2 templates). Tag choices follow
html5_polyglot where there's an obvious correspondence
(section/h1-h6/p/ul/ol/li/blockquote/table/thead/tbody/tr/td/th,
em/strong/code/cite/sub/sup/abbr; a grid-table cell's column/row span
becomes `colspan`/`rowspan`, HTML's own native primitives for exactly
this; an option list becomes a `<dl>`, each item's comma-separated
flags joined into one `<dt>`, e.g. `<dt>-f, --file=FILE</dt>`; a
`:math:` role becomes the raw TeX source wrapped in `\(...\)`, the
MathJax inline-delimiter convention, plain text with no wrapping tag
or script dependency — MathJax auto-detects it with no markup of its
own to hook into); a directive (including a
substitution definition's embedded `replace::`) renders as
`<pre class="directive" data-directive="name">` rather than being
silently dropped, since there's no semantic dispatch to render it
properly; an unresolved reference/substitution-reference falls back to
plain text since there's nothing to link to or substitute. Verified
structurally against docutils' `--writer=html5` output on representative
documents (not byte-for-byte, given the scope above) plus a tag-balance
check over a document exercising every implemented construct together.

```go
import "github.com/go-docutils/docutils/html"

fmt.Println(html.Render(doc)) // e.g. "<p>Hello <em>world</em>.</p>"
```

**`latex`**: `latex.Render(doc) string` renders a doctree to a complete,
standalone, compilable `.tex` document — meant as input to a LaTeX
engine such as [go-tex](https://github.com/go-tex). Unlike `html.Render`
(a fragment meant to be embedded), LaTeX has no equivalent to dropping a
fragment into a hosting page, so a full document — a fixed
`\documentclass{article}` with a minimal preamble (`hyperref` only, for
working links/anchors) — is the useful unit. Also deliberately NOT a
port of docutils' latex2e writer (`writers/latex2e/__init__.py`, ~3486
lines: multiple document classes, syntax-highlighted listings, real
LaTeX `\footnote`-machinery bridged across the doctree's separate
footnote-definition/-reference nodes via custom preamble macros,
docinfo-to-titlepage conversion). This uses only vanilla LaTeX
constructs (`itemize`/`enumerate`/`quote`/`verbatim`/`description`/
`verse`/`tabular`), so it always compiles without a custom macro
package — a field list, a definition list, AND an option list (its
comma-separated flags joined into one `\item[{...}]`) all share the
same `description` environment, since none of the three has a native
LaTeX construct of its own. A `:math:` role renders as core `$...$`
inline math mode — its content is TeX source already, written
verbatim rather than through the usual text-escaping pass, which
would otherwise corrupt the very characters (`^`, `_`, `\`) math mode
depends on. A table's cell content is flattened to
plain text — a nested
list or multi-paragraph cell would need a `p{width}` column + minipage
to stay valid LaTeX, not implemented here. A grid-table cell's column
span renders as `\multicolumn` (plain LaTeX, no package); its ROW span
does NOT — plain `tabular` has no rowspan primitive without the
`multirow` package, which this writer deliberately never depends on, so
a row-spanning cell's content still appears but isn't merged, which can
visually misalign a later row that relied on the merge (real row/column
spans both work correctly in `html.Render`, since HTML has native
primitives for this and no such package constraint). Footnotes/citations don't use
LaTeX's native `\footnote` (it wants inline content at the reference
point, docutils' doctree has them as separate nodes); a reference
renders as a `\hyperlink` jump to a labeled paragraph where the
definition appears in the document's normal flow, not a page-bottom
note. Verified by actually compiling representative output (special
characters, nested sections past LaTeX's 5 native depths, every
implemented construct together) with
[tectonic](https://tectonic-typesetting.github.io/) during development
— real PDFs, zero errors — not just structural comparison; that step
isn't part of `go test` itself since a LaTeX engine isn't a build
dependency of this module (same "reference tool, not a runtime
dependency" rule as the docutils foreign judge).

```go
import "github.com/go-docutils/docutils/latex"

os.WriteFile("out.tex", []byte(latex.Render(doc)), 0644)
// tectonic out.tex  (or any other LaTeX engine, incl. go-tex)
```

## Testing

`go test ./...`. Fixtures in `rst/parser_test.go` were generated from
this parser's own output, then eyeball-verified against the docutils
foreign judge (see the package doc comment) before being frozen — not
hand-transcribed (for footnotes/citations/substitutions, "docutils
foreign judge" means `Parser().parse(src, document)` directly rather
than `publish_string`, to see the tree before docutils' own transforms
run — see the `rst` package doc comment). Coverage as of this writing:
`doctree` 97%, `rst` 93%, `html` 89%, `latex` 87%. `go vet ./...` and
`gofmt -l .` clean.
