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
bullet lists, enumerated lists — all five of docutils' own sequences
(arabic, `loweralpha`/`upperalpha`, `lowerroman`/`upperroman`) in all
three formats (`N.`, `(N)`, `N)`), plus the auto-enumerator `#`, with
docutils' own ambiguity-resolution rules (`Body.parse_enumerator`, read
directly: a bare single roman-charset letter defaults to roman ONLY when
no sequence is already established — "H." then "I." continues an
established `upperalpha` list as ordinal 9, not a fresh `upperroman`
one), a malformed roman numeral (`iiii`, no valid subtractive form)
rejected outright rather than treated as a valid ordinal, `enumtype`/
`prefix`/`suffix`/`start` attributes, and the "start value not
ordinal-1" INFO + "ends without a blank line" WARNING diagnostics (both
land as SIBLINGS of the `<enumerated_list>`, never nested inside it,
matching `self.parent += msg` read directly) — enumerator-sequence
validation WITHIN an already-started list (docutils errors on a
non-consecutive ordinal mid-list; this parser's own continuation check
already requires exact `+1`, so a gap just ends the list instead of
producing that distinct diagnostic) is the one piece of this still not
fully ported, see "Not yet ported" below — field lists
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
per-directive semantics, with these exceptions: `raw` (`.. raw::
FORMAT`, whose content passes through completely unprocessed, tagged
with its target format; see `Options.RawEnabled`, on by default matching
real docutils' own default despite its confusingly-named `--no-raw`
flag), `table` (`.. table:: TITLE`, dispatching its body through this
project's own existing simple/grid table parser and requiring exactly
one table to result — `:class:`/`:name:`/`:align:`/`:width:`/`:widths:`
options, the title itself inline-parsed so it can carry markup or a
dangling-markup warning of its own), `list-table` (building a
`<table>` from scratch out of a uniform two-level bullet list — rows,
each a nested list of cells — plus `:header-rows:`/`:stub-columns:`),
and the nine generic ADMONITIONS — `attention`/`caution`/`danger`/
`error`/`hint`/`important`/`note`/`tip`/`warning`, directive name
matched case-INSENSITIVELY (`.. Note::`, `.. WARNING::` work the same as
`.. note::`), each just wrapping its own content in a like-named node —
plus the one non-generic `admonition` directive (a REQUIRED title
argument becoming a `<title>`, auto-classed `admonition-<slug of the
title>` unless `:class:` overrides it), `topic` (a REQUIRED title
argument), and `sidebar` (an OPTIONAL title argument, plus its own
`:subtitle:` option — valid only alongside a title) — both with a real
NESTING restriction real docutils itself enforces: a topic/sidebar is
only valid directly inside a document or section (a topic ALSO directly
inside a sidebar); anywhere else — a list item, a block quote, another
topic — is an ERROR, checked against this parser's own notion of "the
container currently being appended to", the same thing real docutils'
own `state_machine.node` check means; `:class:`/`:name:` options work
the same way real docutils' own generic option/content-block split does
(`Body.parse_directive_block`, read directly): the option block is
whatever TRAILING run of `:key: value` lines the directive's own body
ends with, wherever that starts — not merely a leading run assumed to be
at the very top, the simpler shape this project's `table`/`list-table`
options still use, since no corpus case has needed the general form
there — so real content is free to precede the options, and a role
invocation elsewhere in that content is never mistaken for one.
Directive-level argument/option-syntax errors, e.g. a malformed
`:widths:` value or an unknown option key entirely, are NOT validated
for any directive, matching this project's own established scope
boundary — see "Not yet ported" below, hyperlink
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
`abbreviation`/`ab`, `acronym`/`ac`), plus its other three always-registered,
non-generic roles: `math` (a dedicated `<math>` node holding the raw,
unescaped TeX source, rendered by both writers below); `code` (real
class-list/highlight-language derivation, `roles.py`'s `code_role`
ported — the content is preserved raw, backslashes intact, wrapped in
`<literal class="...">`; since this parser has no Pygments equivalent
and never will, a *resolved* highlight language always takes docutils'
own `LexerError` path, which itself distinguishes an EXPLICIT
`:language:` option, a WARNING + `<problematic>` — "Cannot analyze code.
Pygments package not found." — from one merely implied by a custom
role's own name, a silent fallback to the same plain, unclassified shape
as no language at all); and `pep`/`pep-reference` and `rfc`/`rfc-reference`
(numeric validation — a PEP number 0-9999, an RFC number ≥ 1, optionally
with a `#section` suffix — producing a `<reference>` to the canonical
page on success or an ERROR + `<problematic>` otherwise), plus a
`.. role:: NAME(BASE)`-registered custom role — aliasing a generic role
by tag the same way a built-in does, `code` (with its own `:language:`
option, see above), a bare definition with no `BASE` at all (docutils'
`generic_custom_role`, a plain `<inline>`), or (`BASE` is `raw`, with a
`:format:` option) this parser's one INLINE raw construct, mirroring the
`raw` directive above (the BUILT-IN `raw` role used directly, with no
`.. role::` registration at all and so no `:format:` reaching it, always
errors instead, matching `roles.py`'s own `raw_role`) — EVERY one of
these carries a `:class:` option (real docutils' `set_implicit_options`:
every role function implicitly supports one), defaulting to the role's
own name exactly like real docutils' `Role` directive does, rendered as
a `class="..."` attribute on whatever element the role produces — a
registered custom role's `<inline class="...">` is NOT the same shape as
a totally unregistered role name, which still falls back to this
parser's own invented `<inline role="name">` rather than docutils'
error (see "Not yet ported" below), and
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
the exact list and why): the "ends without a blank line; unexpected
unindent" WARNING real docutils' own `unindent_warning` produces for
EVERY explicit-markup construct (a directive, footnote, citation, ...)
that a following, insufficiently-indented line interrupts — this parser
only has it for enumerated lists so far (parser.go, ported alongside
v0.25.0's full enumerated-list work); a topic/sidebar containing a
rejected nested directive, for instance, is missing this one diagnostic
even though the rejection itself is correctly reported. Docutils'
*standalone* PEP/RFC recognition —
bare `pep-123`/`RFC 123` text with no `:PEP:`/`:RFC:` role markup at all
(checked against `Parser().parse()` with default settings — docutils'
own `pep_references`/`rfc_references` settings default to **off**, so
implementing this unconditionally would diverge from upstream's own
default rather than fill a real gap; the `:PEP:`/`:RFC:` roles
themselves ARE ported, see above), and an unknown
interpreted-text role's rewrite to `problematic` — deliberately, not for
lack of the machinery: this parser has a real role registry now (see
`.. role::` below), which resolves exactly what it needs to (a custom
role's own `raw` indirection), but this project chose not to also start
rewriting every OTHER unrecognized name to `problematic`, since that
would be a real leniency REGRESSION for any document using a role this
parser has simply never heard of (a Sphinx/extension role, say) rather
than a gap filled — real docutils always errors there, this parser
still doesn't, on purpose. Directive-level argument/option-syntax
validation is likewise not implemented for ANY directive (an
unresolvable role base, an invalid option value, a malformed directive
argument list, a required argument missing entirely — real docutils
raises a distinct diagnostic for each; this parser's own directives,
`raw`/`table`/`list-table`, silently ignore an option they don't
understand rather than erroring, matching the same leniency choice as
the unknown-role case just above) — a real, general gap, not chased
per-directive as each one gets implemented. Title-style consistency (a title style's LEVEL
is fixed by first-seen order across the whole document; skipping more
than one new level at once is an error) and section-title diagnostics
(too-short overline/underline, missing/mismatched underline, incomplete
title, invalid title-or-transition) ARE enforced — `matchTitle`/
`titleDiagnostic`/`checkSubsectionLevel`, `rst/parser.go`; the
`match_titles=false` case (a title-looking construct somewhere titles
aren't allowed, e.g. inside a block quote) and enumerator-sequence
validation (docutils errors on a non-consecutive ordinal) are the two
pieces still NOT ported. A genuinely separate, deliberately NOT chased
gap found alongside the enumerator work: once a line has fallen through
every recognized block-construct check to become ordinary paragraph
text, real docutils' own paragraph gathering (`Text.text`'s
`get_text_block`, read directly) swallows every subsequent line
unconditionally up to the next blank line or dedent — it never
re-examines a LATER line to see whether it independently looks like a
different construct. This parser's own paragraph-gathering
(`consumeParagraph`) does the opposite: it still stops early whenever a
later line matches a recognized marker shape, even mid-paragraph — a
real, if narrow, architectural difference from real docutils that spans
every block-construct check there (bullet/field/doctest/table/etc.), not
just enumerators; fixing it properly is a bigger, separate undertaking
than this round's own enumerator scope. A table's column-margin violations are never detected
(only the "last column overflows its width" case is handled, since real
content relies on it). A block quote's own indent is discovered the same
way real docutils' `StringList.get_indented` does: the MINIMUM across a
whole (possibly variable-depth) indented run, not the first line's own
indent — a deeper-then-shallower run correctly NESTS instead of producing
sibling block quotes. A trailing "-- text" / "--- text" / em-dash-prefixed
attribution line, preceded by a blank line and internally
consistently-indented, becomes a real `<attribution>`, splitting the
region into one `<block_quote>` per attribution boundary — `split_attribution`
+ `check_attribution`, ported; the diagnostics real docutils emits for a
malformed attempt (an inconsistent-indent continuation, an unindent with no
blank line first) are deliberately NOT ported, the same scope boundary as
title-style/table-column diagnostics just above. A dangling NAMED reference (bare, backtick-quoted,
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
document becomes `<problematic>`, all sharing ONE message. An unclosed
inline-markup start-string (`*x`, `**x`, two backticks with no closing
pair, a bare interpreted-text backquote) IS rewritten to `<problematic>`
too — `Inliner.inline_obj`, ported — a genuinely SEPARATE source from the
dangling-reference/anonymous-mismatch cases above: this one fires during
inline PARSING itself (`inline.go`), not a whole-document post-pass over
an already-built tree, and its `<system_message>` is attached directly as
a sibling of whatever it came from (a paragraph, a section title, a block
quote's attribution, a field name's body, a definition list term, a line
block line) at that construct's own point of origin — real docutils'
`RSTState.paragraph`/`new_subsection`/`parse_attribution`/`field`/
`line_block_line`/`term` all `return`/`+=` these messages as siblings,
states.py read directly — NEVER collected into the trailing
`system-messages` section above. That section is real docutils' own
`transforms.universal.Messages`, and it wraps only messages with no
parent at all (`if not msg.parent`, read directly) — an inline-markup
message already has one the moment it's attached, so it is categorically
excluded, unlike the dangling-reference/anonymous-mismatch messages
(built via `document.reporter.error` with no tree insertion of their
own, hence genuinely parentless). A `substitution_reference` ("`|x`" with no closing "`|`") routes through
the identical real-docutils mechanism (`inline_obj`) yet never actually
produces this warning in practice — checked against the foreign judge for
several inputs, not assumed from reading the source alone — so this parser
matches that observed behavior rather than second-guessing it with a
warning real docutils itself doesn't emit.

Whether a `*`/`**`/two-backtick/backquote/`|` counts as a genuine markup
start- or end-string at all is docutils' own `start_string_prefix`/
`end_string_suffix` rule, ported verbatim (`punctuation.go`) — not "any
punctuation on either side", this parser's own earlier, simpler
approximation via Go's `unicode.IsPunct`, which treated an opening and a
closing bracket/quote identically. Real docutils distinguishes them: a
start-string may be preceded only by whitespace, an OPENER, or a
DELIMITER (`(*emphasis*)`, `-*emphasis*-`), never a CLOSER or a
CLOSING-DELIMITER (`)*emphasis*(` is NOT markup at all); an end-string may
be followed only by whitespace, a CLOSER, a DELIMITER, or a
CLOSING-DELIMITER (`*emphasis*.`, `*emphasis*)`), never an OPENER
(`*emphasis*(` is unclosed, not valid markup). A markup start-string
immediately sandwiched between a real matching open/close pair with
nothing else between (`(*)text`) is additionally rejected —
`Inliner.quoted_start`, ported — even though the basic rule alone would
accept it. The four character classes (openers/closers/delimiters/
closing-delimiters) and the open/close quote-pairing table are
`docutils.utils.punctuation_chars` verbatim, covering the full range of
quotation-mark conventions real docutils itself supports (French, German,
Polish, Hungarian, Greek, CJK, ...) — generated from a live Python
reference and cross-checked against every Unicode code point, not
hand-transcribed (a first attempt using literal glyphs silently
substituted at least two visually-similar-but-distinct characters). A
backslash-escaped space or newline immediately after an end-string is
itself a valid boundary (docutils' own `\x00` escape-marker, which
`end_string_suffix` explicitly allows) and is dropped from the resulting
text entirely rather than rendered as a literal space — docutils' own
`unescape()`, "backslash-escaped spaces are also removed" — a real bug
this parser had for every kind of escaped whitespace, not just at a
markup boundary, only exposed once markup started closing correctly for
inputs that exercise it. The interpreted-text backquote and the
substitution-reference `|` each allow an optional trailing `_`/`__` to be
consumed by the SAME end-string-suffix check as part of resolving them as
references (`` `text`_ ``, `|sub|__`) — a separate regex in real docutils
from the generic emphasis/strong/literal one, ported as its own boundary
check (`findCloseBackquote`/`findCloseSubstitution`) rather than folded
into the shared one, since only these two constructs have it. An unknown
interpreted-text role still stays a plain node instead — deliberately,
see above. A resolved
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
output). A literal span (`` `` `` ``) RESTORES its backslash escapes
instead of stripping them — the ONLY marker with this behavior:
`Inliner.literal` calls `inline_obj(..., restore_backslashes=True)`,
which (`nodes.unescape`, read directly) puts the literal backslash back
rather than dropping it, so `` ``\literal`` `` keeps its own visible
backslash (`\literal`) where every other construct (emphasis, strong,
a role's own interpreted text, ...) would silently drop it. An escaped
character that would otherwise complete a MULTI-CHARACTER close
delimiter (an escaped first backtick immediately followed by a second,
real one — `` ``text\`` `` — real docutils' `\x00`-substitution still
counts the escaped character's own identity toward the "``" pair, only
suppressing the backslash from the final text) is a known, NOT yet
ported gap: this parser's own escaped-rune representation
(`escapeRune`/`isEscapedRune` in `inline.go`) makes an escaped character
unrecognizable as itself to any literal-rune comparison, which also
affects general marker start-boundary recognition immediately after a
backslash-escaped space (`m\ *a*` — the escaped space should count as
valid preceding whitespace for `*a*` to open at all, same underlying
cause) — confirmed against the foreign judge, deliberately NOT fixed in
this round: it touches boundary-matching code shared by every marker
type, not a single self-contained call site.

Every `<system_message>` this parser builds now carries `level`/`type`
(constants per message kind — inline-markup-time messages are always
`level="2" type="WARNING"`, matching `Inliner.inline_obj`'s
`self.reporter.warning(...)` exactly, since "start-string without
end-string" is the only text that function ever produces) and, for a
PARAGRAPH or a SECTION TITLE at the top level of the document (i.e. not
nested inside a list item, block quote, field body, definition, or
table cell), `line` too — the construct's own real 1-indexed source
line, matching real docutils' `RSTState.paragraph`/`new_subsection`
exactly (verified against the foreign judge for both an underlined and
an overlined title, where the reported line is the title TEXT's own
line, never the overline's). A message from any OTHER inline-parsing
context, or from a nested one, still omits `line` — this parser doesn't
track absolute source position through recursion into a rebased
sub-slice of lines (a list item, a block quote's body, a field's body,
a definition, a table cell) anywhere yet, a genuinely separate,
larger undertaking (see `parser.currentLine`'s own doc comment in
`parser.go`) than threading it through the two top-level call sites
this round covers. A table's `<tgroup cols="N">`/`<colspec colwidth="W">` wrapper
IS produced (verified: it's part of the bare parse, not a later
transform or writer-side addition, unlike the above), the `html`/
`latex` writers and `go-richdoc/rst` all unwrap it transparently.
Every section title IS registered as an implicit hyperlink target too
(docutils' own `new_subsection`/`create_id`, ported): a `` `Some Title`_ ``
reference resolves to a same-document anchor derived from the title, the
`id` a plain-ASCII slug (accents folded, everything else stripped) and the
`name` the whitespace-normalized title text — both writers emit the `id`
as a real anchor (`<section id="...">` in HTML, a bare `\hypertarget{id}{}`
right before the sectioning command in LaTeX). Two sections sharing a
title get distinct ids (`title`, `title-1`, ...) with no ambiguous-name
diagnostic, the same "no duplicate/ambiguous-name diagnostics"
simplification as dangling-reference rewriting above.

Section-title recognition and its diagnostics are ported from real
docutils' `Line`/`Text` states (`states.py`, read directly), not just
the well-formed case: a title inset under its overline (leading
whitespace on the title line) is stripped rather than rejected, but
still counts toward the overline-width comparison exactly as docutils
computes it (before the strip, not after — an inset title can trigger
"Title overline too short." on its own even when the stripped text
alone would fit); a too-short overline or underline (under 4 columns)
that's ALSO narrower than the title reverts the whole attempt to plain
text with an INFO notice, while one that's merely narrower (but still
≥4, or ≥ the title's own width) is a WARNING and the section is still
created; a missing, mismatched, or absent-at-EOF underline, and two
overlines with no title text between, are each their own ERROR with no
section created. Title-STYLE consistency is enforced too: a style's
level is fixed by the order it's first seen in the whole document
(`title_styles`, ported), reusing an established style returns to that
level (closing any deeper-nested sections), and introducing more than
one new level at once — skipping a level — is
`"Inconsistent title style: skip from level X to Y."`, an ERROR with no
section created. A numbered line ("`1. Numbered Title`") is
disambiguated from a genuine enumerated-list item by peeking at the
following line (`is_enumerated_list_item`, ported): blank, indented, or
starting with the next ordinal's own marker confirms a real list item;
anything else — most commonly a title underline — corrects the
enumerator-looking line back to plain text, letting the title win.
**Not yet ported**: the `match_titles=False` diagnostics for a title-
looking construct found somewhere titles aren't allowed at all (inside
a block quote or list item — real docutils still errors there,
`"Unexpected section title."` or `"...or transition."`; this parser
currently treats it as plain text with no diagnostic), and enumerator-
sequence validation (docutils warns on a non-consecutive ordinal; this
parser doesn't check).

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
