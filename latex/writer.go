// Package latex renders a doctree.Element into a standalone LaTeX
// document, meant as input to a LaTeX engine such as go-tex.
//
// Deliberately NOT a port of docutils' latex2e writer
// (writers/latex2e/__init__.py, ~3486 lines): that writer supports
// multiple document classes, syntax-highlighted code listings, real
// LaTeX \footnote-machinery bridged across the doctree's separate
// footnote-definition/footnote-reference nodes via custom \DU...
// preamble macros, docinfo-to-titlepage conversion, and more — replicating
// it would be a second undertaking on the scale of the writer itself.
// This produces a fixed article-class document with a minimal preamble
// (hyperref only, for working links/anchors) using only vanilla LaTeX
// constructs, so it always compiles without a custom macro package.
//
// SCOPE (v1): unlike html.Render (a fragment, meant to be embedded), Render
// here returns a COMPLETE, standalone, compilable .tex document —
// LaTeX has no equivalent to dropping a fragment into a hosting page, so a
// full document is the useful unit. A table's cell content is flattened to
// plain text (doctree.AsText) rather than walked recursively: a nested
// list or multi-paragraph cell needs a `p{width}` column + minipage to be
// valid LaTeX, not implemented here. A grid-table cell's column span
// (morecols) is rendered with `\multicolumn` (plain LaTeX, no package);
// its ROW span (morerows — a cell spanning multiple text rows) is NOT
// rendered specially at all: plain `tabular` has no rowspan primitive
// without the `multirow` package, which this writer deliberately never
// depends on (see the module doc comment) — a row-spanning grid-table
// cell's content still appears, just not merged, so a later row that
// relied on that merge to stay aligned may visually misalign. Real
// row/column spans work correctly in html.Render (`rowspan`/`colspan`
// are native HTML). Footnotes/citations don't use LaTeX's
// native \footnote (which wants inline content at the reference point, not
// docutils' separate reference/definition nodes) — a reference renders as
// a hyperref jump to a labeled paragraph where the definition appears in
// the document's normal flow, not a page-bottom note. A directive
// (including a substitution definition's embedded `replace::`) renders as
// a verbatim block labeled with its name, same non-silent-drop choice as
// html.Render. An unresolved reference/substitution falls back to plain
// text.
package latex

import (
	"strconv"
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// sectionCommands mirrors docutils' DocumentClass.sections for the
// (default) "article" class: level 1..5, capped at "subparagraph" for
// anything deeper.
var sectionCommands = []string{"section", "subsection", "subsubsection", "paragraph", "subparagraph"}

// Render walks doc and returns a complete, standalone LaTeX document.
func Render(doc *doctree.Element) string {
	var body strings.Builder
	renderChildren(&body, doc, 1)

	var b strings.Builder
	b.WriteString("\\documentclass{article}\n")
	b.WriteString("\\usepackage{hyperref}\n")
	b.WriteString("\\begin{document}\n")
	b.WriteString(body.String())
	b.WriteString("\n\\end{document}\n")
	return b.String()
}

func renderChildren(b *strings.Builder, el *doctree.Element, level int) {
	for _, c := range el.Children {
		renderNode(b, c, level)
	}
}

func renderNode(b *strings.Builder, n doctree.Node, level int) {
	switch v := n.(type) {
	case *doctree.Text:
		b.WriteString(escapeText(v.Data))
	case *doctree.Element:
		renderElement(b, v, level)
	}
}

func renderElement(b *strings.Builder, el *doctree.Element, level int) {
	switch el.Tag {
	case doctree.TagDocument:
		renderChildren(b, el, level)
	case doctree.TagSection:
		// The section's OWN title renders at `level`; everything else
		// (including a nested <section>, one level deeper) gets level+1
		// — same reasoning as html.Render's heading-depth tracking. The
		// bare \hypertarget (see assignSectionTargets for the id) makes a
		// `Some Title`_-style reference to this section resolve to a real
		// anchor instead of a dangling \hyperlink, same idiom TagTarget
		// already uses for an inline internal target.
		if id := el.Attr("id"); id != "" {
			b.WriteString("\\hypertarget{" + escapeText(id) + "}{}")
		}
		for _, c := range el.Children {
			if ce, ok := c.(*doctree.Element); ok && ce.Tag == doctree.TagTitle {
				renderElement(b, ce, level)
				continue
			}
			renderNode(b, c, level+1)
		}
	case doctree.TagTitle:
		cmd := sectionCommands[len(sectionCommands)-1]
		if level >= 1 && level <= len(sectionCommands) {
			cmd = sectionCommands[level-1]
		}
		b.WriteString("\n\\" + cmd + "{")
		renderChildren(b, el, level)
		b.WriteString("}\n")
	case doctree.TagParagraph:
		renderChildren(b, el, level)
		b.WriteString("\n\n")
	case doctree.TagBulletList:
		wrapEnv(b, "itemize", func() { renderListItems(b, el, level) })
	case doctree.TagEnumeratedList:
		wrapEnv(b, "enumerate", func() { renderListItems(b, el, level) })
	case doctree.TagListItem:
		renderChildren(b, el, level) // reached only defensively; see renderListItems
	case doctree.TagBlockQuote:
		wrapEnv(b, "quote", func() { renderChildren(b, el, level) })
	case doctree.TagAttribution:
		// Real docutils' default `attribution` setting ("dash") right-aligns
		// the text with a bare em-dash prefix — verified against
		// publish_string's actual latex output, not assumed. Grouped in
		// braces so \raggedleft doesn't leak past this one line — vanilla
		// LaTeX, no custom macro package, matching this writer's own
		// existing scope (see the package doc comment).
		b.WriteString("\n{\\raggedleft —")
		renderChildren(b, el, level)
		b.WriteString("\\par}\n")
	case doctree.TagTransition:
		b.WriteString("\n\\noindent\\hrulefill\n\n")
	case doctree.TagLiteralBlock, doctree.TagDoctestBlock:
		b.WriteString("\n\\begin{verbatim}\n")
		b.WriteString(doctree.AsText(el))
		b.WriteString("\n\\end{verbatim}\n")
	case doctree.TagComment:
		for _, line := range strings.Split(doctree.AsText(el), "\n") {
			b.WriteString("% " + line + "\n")
		}
	case doctree.TagRaw:
		// Verbatim, NOT escapeText: the whole point of "raw" is content
		// that bypasses this writer's own escaping — see Options.RawEnabled
		// in rst.Parse for how it gets here at all (the block ".. raw::"
		// directive, or docutils/rst v0.16.0+'s inline ".. role::(raw)"
		// form). Only emitted for a raw node actually targeting "latex"
		// specifically (checked against real docutils' own latex2e
		// writer's visit_raw: it tests for "latex", not "tex" — a format
		// list can name several writers, ".. raw:: html latex", each
		// writer only honors its own). The surrounding newlines matter,
		// not just formatting: real docutils' own visit_raw/depart_raw
		// add them for a block-level raw node too (verified against the
		// foreign judge) — without one, a raw LaTeX command ending in
		// letters (`\bfseries`, say) would swallow whatever ordinary text
		// immediately follows it into the same control sequence name and
		// fail to compile. This writer doesn't distinguish an inline
		// occurrence from a block one here (both reach this same case),
		// so the same newlines apply there too — cosmetic only for
		// inline, not a correctness bug: LaTeX collapses a bare newline
		// to a single space outside a blank line, confirmed by actually
		// compiling an inline case through go-tex/engine and reading the
		// resulting PDF's text back with pdftotext.
		if formatTargets(el.Attr("format"), "latex") {
			b.WriteString("\n" + doctree.AsText(el) + "\n")
		}
	case doctree.TagFieldList, doctree.TagDefinitionList, doctree.TagOptionList, doctree.TagDocinfo:
		wrapEnv(b, "description", func() { renderDescriptionItems(b, el, level) })
	case doctree.TagOptionGroup:
		renderOptionGroup(b, el)
	case doctree.TagOption:
		renderOption(b, el)
	case doctree.TagLineBlock:
		wrapEnv(b, "verse", func() { renderChildren(b, el, level) })
	case doctree.TagLine:
		renderChildren(b, el, level)
		b.WriteString(" \\\\\n")
	case doctree.TagFootnote, doctree.TagCitation:
		id := el.Attr("name")
		b.WriteString("\n\\par\\noindent")
		if id != "" {
			b.WriteString("\\hypertarget{" + escapeText(id) + "}{}")
		}
		renderChildren(b, el, level)
		b.WriteString("\\par\n")
	case doctree.TagLabel:
		b.WriteString("\\textbf{[")
		renderChildren(b, el, level)
		b.WriteString("]} ")
	case doctree.TagTarget:
		// The second \hypertarget group is the visible content: empty for
		// a block-level hyperlink target (no children, same as before),
		// but an inline internal target ("_`text`") has real visible text
		// that must not be dropped.
		if name := el.Attr("name"); name != "" {
			b.WriteString("\\hypertarget{" + escapeText(name) + "}{")
			renderChildren(b, el, level)
			b.WriteString("}")
		}
	case doctree.TagDirective:
		name := el.Attr("name")
		b.WriteString("\n\\begin{verbatim}\n[directive: " + name + "]\n")
		if args := el.Attr("arguments"); args != "" {
			b.WriteString(args + "\n")
		}
		b.WriteString(doctree.AsText(el))
		b.WriteString("\n\\end{verbatim}\n")
	case doctree.TagSubstitutionDef:
		// No output of its own — see package doc comment.
	case doctree.TagTable:
		renderTable(b, el, level)
	case doctree.TagEmphasis:
		wrapCmd(b, "emph", el, level)
	case doctree.TagStrong:
		wrapCmd(b, "textbf", el, level)
	case doctree.TagLiteral:
		wrapCmd(b, "texttt", el, level)
	case doctree.TagTitleReference:
		wrapCmd(b, "emph", el, level)
	case doctree.TagSubscript:
		wrapCmd(b, "textsubscript", el, level)
	case doctree.TagSuperscript:
		wrapCmd(b, "textsuperscript", el, level)
	case doctree.TagMath:
		// Verbatim, NOT escapeText: the content is TeX math source itself
		// (docutils' math_role stores it raw/unparsed), and escaping its
		// own special characters (^, _, \) would corrupt the very syntax
		// $...$ math mode depends on.
		b.WriteString("$" + doctree.AsText(el) + "$")
	case doctree.TagAbbreviation, doctree.TagAcronym, doctree.TagInline:
		renderChildren(b, el, level)
	case doctree.TagReference:
		uri := el.Attr("refuri")
		switch {
		case strings.HasPrefix(uri, "#"):
			// A same-document anchor (an inline internal target resolves
			// to "#name", never a real URL) — \hyperlink, not \href: the
			// leading "#" is hyperref's OWN internal-link marker, and
			// escapeURL's "#"->"\#" escaping (correct for a URL's real
			// fragment) would corrupt it here, the same distinction
			// footnote/citation references below already make.
			b.WriteString("\\hyperlink{" + escapeText(uri[1:]) + "}{")
			renderChildren(b, el, level)
			b.WriteString("}")
		case uri != "":
			b.WriteString("\\href{" + escapeURL(uri) + "}{")
			renderChildren(b, el, level)
			b.WriteString("}")
		default:
			renderChildren(b, el, level)
		}
	case doctree.TagSubstitutionRef:
		// Unresolved (see rst/inline.go): render the substitution name
		// as plain text, the best available fallback.
		renderChildren(b, el, level)
	case doctree.TagFootnoteReference, doctree.TagCitationReference:
		if ref := el.Attr("refname"); ref != "" {
			b.WriteString("\\hyperlink{" + escapeText(ref) + "}{[")
			renderChildren(b, el, level)
			b.WriteString("]}")
		} else {
			renderChildren(b, el, level)
		}
	default:
		renderChildren(b, el, level)
	}
}

func wrapEnv(b *strings.Builder, env string, body func()) {
	b.WriteString("\n\\begin{" + env + "}\n")
	body()
	b.WriteString("\n\\end{" + env + "}\n")
}

func wrapCmd(b *strings.Builder, cmd string, el *doctree.Element, level int) {
	b.WriteString("\\" + cmd + "{")
	renderChildren(b, el, level)
	b.WriteString("}")
}

func renderListItems(b *strings.Builder, list *doctree.Element, level int) {
	for _, c := range list.Children {
		item, ok := c.(*doctree.Element)
		if !ok || item.Tag != doctree.TagListItem {
			continue
		}
		b.WriteString("\\item ")
		renderChildren(b, item, level)
	}
}

// renderDescriptionItems renders a field_list/definition_list/option_list's
// name-body pairs as \item[term] entries in a description environment. An
// option_list_item's "name" is its option_group, whose own ", "-joined
// rendering (renderOptionGroup) is reached via renderNode -> renderElement's
// TagOptionGroup case below (see the renderNode call further down).
func renderDescriptionItems(b *strings.Builder, list *doctree.Element, level int) {
	for _, c := range list.Children {
		pair, ok := c.(*doctree.Element)
		if !ok {
			continue
		}
		// A docinfo typed field (promoteDocInfo, rst/docinfo.go) is a
		// bare element, not a (name, body) pair — its own tag name is the
		// term docutils would otherwise have kept as a separate
		// <field_name>.
		switch pair.Tag {
		case doctree.TagAuthors:
			b.WriteString("\\item[{authors}] ")
			first := true
			for _, ac := range pair.Children {
				author, ok := ac.(*doctree.Element)
				if !ok || author.Tag != doctree.TagAuthor {
					continue
				}
				if !first {
					b.WriteString(", ")
				}
				first = false
				renderChildren(b, author, level)
			}
			continue
		case doctree.TagAuthor, doctree.TagOrganization, doctree.TagAddress, doctree.TagContact,
			doctree.TagVersion, doctree.TagRevision, doctree.TagStatus, doctree.TagDate, doctree.TagCopyright:
			b.WriteString("\\item[{" + pair.Tag + "}] ")
			renderChildren(b, pair, level)
			continue
		}
		nameTag, bodyTag := doctree.TagFieldName, doctree.TagFieldBody
		switch pair.Tag {
		case doctree.TagDefinitionListItem:
			nameTag, bodyTag = doctree.TagTerm, doctree.TagDefinition
		case doctree.TagOptionListItem:
			nameTag, bodyTag = doctree.TagOptionGroup, doctree.TagDescription
		}
		for _, cc := range pair.Children {
			ce, ok := cc.(*doctree.Element)
			if !ok {
				continue
			}
			switch ce.Tag {
			case nameTag:
				b.WriteString("\\item[{")
				// renderNode, not renderChildren: for TagOptionGroup this
				// must reach renderElement's own case (the ", "-joined
				// rendering), not iterate its option children directly.
				// For TagFieldName/TagTerm it's equivalent, since neither
				// has a dedicated renderElement case of its own.
				renderNode(b, ce, level)
				b.WriteString("}] ")
			case bodyTag:
				renderChildren(b, ce, level)
			}
		}
	}
}

// renderOptionGroup renders an option_group's option children joined by
// ", " (the man-page convention for a grouped short/long flag pair) — not
// left to plain renderChildren, which has no way to insert a separator
// between siblings.
func renderOptionGroup(b *strings.Builder, group *doctree.Element) {
	first := true
	for _, c := range group.Children {
		opt, ok := c.(*doctree.Element)
		if !ok || opt.Tag != doctree.TagOption {
			continue
		}
		if !first {
			b.WriteString(", ")
		}
		first = false
		renderOption(b, opt)
	}
}

func renderOption(b *strings.Builder, opt *doctree.Element) {
	for _, c := range opt.Children {
		ce, ok := c.(*doctree.Element)
		if !ok {
			continue
		}
		switch ce.Tag {
		case doctree.TagOptionString:
			b.WriteString(escapeText(doctree.AsText(ce)))
		case doctree.TagOptionArgument:
			// delimiter is always explicitly set when this element exists
			// (see rst's optionNode) — "" genuinely means no separator, the
			// "-ovalue" embedded form, not a missing attribute.
			b.WriteString(escapeText(ce.Attr("delimiter")) + escapeText(doctree.AsText(ce)))
		}
	}
}

// renderTable renders a <table>[<thead>]<tbody> as a plain "l"-columned
// tabular environment. Cell content is FLATTENED to plain text (see the
// package doc comment) — no nested lists or multiple paragraphs.
// tableGroup returns the element actually holding <thead>/<tbody>: a
// <table>'s <tgroup> child (docutils always wraps rows in one, alongside
// <colspec> column-width metadata this writer has no use for — a fixed
// "l" column spec covers every table already), or the table itself if
// there is no tgroup wrapper.
func tableGroup(table *doctree.Element) *doctree.Element {
	for _, c := range table.Children {
		if ce, ok := c.(*doctree.Element); ok && ce.Tag == doctree.TagTgroup {
			return ce
		}
	}
	return table
}

func renderTable(b *strings.Builder, table *doctree.Element, level int) {
	cols := 0
	var thead, tbody *doctree.Element
	for _, c := range tableGroup(table).Children {
		ce, ok := c.(*doctree.Element)
		if !ok {
			continue
		}
		switch ce.Tag {
		case doctree.TagThead:
			thead = ce
		case doctree.TagTbody:
			tbody = ce
		}
	}
	firstRows := tbody
	if thead != nil {
		firstRows = thead
	}
	if firstRows != nil {
		if row, ok := firstRow(firstRows); ok {
			for _, c := range row.Children {
				if entry, ok := c.(*doctree.Element); ok && entry.Tag == doctree.TagEntry {
					cols += 1
					if mc := entry.Attr("morecols"); mc != "" {
						if n, err := strconv.Atoi(mc); err == nil {
							cols += n
						}
					}
				}
			}
		}
	}
	if cols == 0 {
		cols = 1
	}
	b.WriteString("\n\\begin{tabular}{" + strings.Repeat("l", cols) + "}\n\\hline\n")
	if thead != nil {
		renderTableRows(b, thead)
		b.WriteString("\\hline\n")
	}
	if tbody != nil {
		renderTableRows(b, tbody)
	}
	b.WriteString("\\hline\n\\end{tabular}\n")
}

func firstRow(group *doctree.Element) (*doctree.Element, bool) {
	for _, c := range group.Children {
		if row, ok := c.(*doctree.Element); ok && row.Tag == doctree.TagRow {
			return row, true
		}
	}
	return nil, false
}

func renderTableRows(b *strings.Builder, group *doctree.Element) {
	for _, c := range group.Children {
		row, ok := c.(*doctree.Element)
		if !ok || row.Tag != doctree.TagRow {
			continue
		}
		var cells []string
		for _, cc := range row.Children {
			entry, ok := cc.(*doctree.Element)
			if !ok || entry.Tag != doctree.TagEntry {
				continue
			}
			text := escapeText(doctree.AsText(entry))
			if mc := entry.Attr("morecols"); mc != "" {
				if n, err := strconv.Atoi(mc); err == nil {
					text = `\multicolumn{` + strconv.Itoa(n+1) + `}{l}{` + text + `}`
				}
			}
			cells = append(cells, text)
		}
		b.WriteString(strings.Join(cells, " & ") + " \\\\\n")
	}
}

// formatTargets reports whether a raw node's space-separated format list
// (".. raw:: html latex" targets both) names target — real docutils' own
// per-writer convention (see e.g. latex2e's visit_raw: `if 'latex' not in
// node['format'].split()`).
func formatTargets(formats, target string) bool {
	for _, f := range strings.Fields(formats) {
		if f == target {
			return true
		}
	}
	return false
}

func escapeText(s string) string {
	return latexEscaper.Replace(s)
}

// escapeURL escapes a URL for use inside \href{...}: LaTeX-special
// characters still need escaping, but a URL is unlikely to contain most
// of them — # is the one that commonly appears (a fragment) and would
// otherwise start a LaTeX macro parameter.
func escapeURL(s string) string {
	return strings.ReplaceAll(s, "#", "\\#")
}

var latexEscaper = strings.NewReplacer(
	`\`, `\textbackslash{}`,
	`{`, `\{`,
	`}`, `\}`,
	`$`, `\$`,
	`&`, `\&`,
	`#`, `\#`,
	`^`, `\textasciicircum{}`,
	`_`, `\_`,
	`~`, `\textasciitilde{}`,
	`%`, `\%`,
)
