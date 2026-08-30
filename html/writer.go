// Package html renders a doctree.Element into an HTML fragment.
//
// This is a semantic-HTML-fragment writer, not a port of docutils'
// html5_polyglot writer (docutils' writers/_html_base.py +
// html5_polyglot/__init__.py total ~2300 lines and embed a full default
// CSS stylesheet, meta/doctype/head boilerplate, and CSS-class
// conventions Sphinx themes build on — replicating that byte-for-byte
// would be a second undertaking on the scale of the whole parser, for a
// stylesheet Sphinx doesn't even use — it has its own Jinja2 templates).
// SCOPE (v1): Render produces a BODY-CONTENT FRAGMENT only — no
// <!DOCTYPE>, <html>, <head>, or stylesheet; embedding it in a page is
// left to the caller. Tag choices follow docutils' html5_polyglot where
// there's an obvious correspondence (section/p/ul/ol/li/blockquote/
// table/thead/tbody/tr, em/strong/code/cite/sub/sup/abbr), but CSS
// classes, ids, and other presentational attributes are NOT replicated
// — this parser doesn't run docutils' id-assignment pass, so there's
// nothing faithful to assign anyway.
package html

import (
	"strconv"
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// Render walks doc and returns an HTML fragment for its content.
func Render(doc *doctree.Element) string {
	var b strings.Builder
	renderChildren(&b, doc, 1)
	return b.String()
}

func renderChildren(b *strings.Builder, el *doctree.Element, headingLevel int) {
	for _, c := range el.Children {
		renderNode(b, c, headingLevel)
	}
}

func renderNode(b *strings.Builder, n doctree.Node, headingLevel int) {
	switch v := n.(type) {
	case *doctree.Text:
		b.WriteString(escapeText(v.Data))
	case *doctree.Element:
		renderElement(b, v, headingLevel)
	}
}

func renderElement(b *strings.Builder, el *doctree.Element, headingLevel int) {
	switch el.Tag {
	case doctree.TagDocument:
		renderChildren(b, el, headingLevel)
	case doctree.TagSection:
		// A section's OWN title renders at the current heading level;
		// everything else (including a nested <section>, whose title
		// must render one level deeper) gets headingLevel+1.
		b.WriteString("<section>")
		for _, c := range el.Children {
			if ce, ok := c.(*doctree.Element); ok && ce.Tag == doctree.TagTitle {
				renderElement(b, ce, headingLevel)
				continue
			}
			renderNode(b, c, headingLevel+1)
		}
		b.WriteString("</section>")
	case doctree.TagTitle:
		h := headingLevel
		if h < 1 {
			h = 1
		}
		if h > 6 {
			h = 6
		}
		tag := "h" + strconv.Itoa(h)
		writeTag(b, tag, "", el, headingLevel)
	case doctree.TagParagraph:
		writeTag(b, "p", "", el, headingLevel)
	case doctree.TagBulletList:
		writeTag(b, "ul", "", el, headingLevel)
	case doctree.TagEnumeratedList:
		writeTag(b, "ol", "", el, headingLevel)
	case doctree.TagListItem:
		writeTag(b, "li", "", el, headingLevel)
	case doctree.TagBlockQuote:
		writeTag(b, "blockquote", "", el, headingLevel)
	case doctree.TagTransition:
		b.WriteString("<hr>")
	case doctree.TagLiteralBlock, doctree.TagDoctestBlock:
		b.WriteString("<pre><code>")
		b.WriteString(escapeText(doctree.AsText(el)))
		b.WriteString("</code></pre>")
	case doctree.TagComment:
		b.WriteString("<!-- ")
		b.WriteString(strings.ReplaceAll(doctree.AsText(el), "--", "- -"))
		b.WriteString(" -->")
	case doctree.TagFieldList, doctree.TagDefinitionList:
		writeTag(b, "dl", "", el, headingLevel)
	case doctree.TagField:
		renderDefinitionPair(b, el, doctree.TagFieldName, doctree.TagFieldBody, headingLevel)
	case doctree.TagDefinitionListItem:
		renderDefinitionPair(b, el, doctree.TagTerm, doctree.TagDefinition, headingLevel)
	case doctree.TagLineBlock:
		writeTag(b, "div", ` class="line-block"`, el, headingLevel)
	case doctree.TagLine:
		writeTag(b, "div", "", el, headingLevel)
	case doctree.TagFootnote, doctree.TagCitation:
		id := el.Attr("name")
		attrs := ` class="footnote"`
		if id != "" {
			attrs += ` id="` + escapeAttr(id) + `"`
		}
		writeTag(b, "div", attrs, el, headingLevel)
	case doctree.TagLabel:
		writeTag(b, "span", ` class="label"`, el, headingLevel)
	case doctree.TagTarget:
		if name := el.Attr("name"); name != "" {
			b.WriteString(`<a id="` + escapeAttr(name) + `"></a>`)
		}
	case doctree.TagDirective:
		name := el.Attr("name")
		b.WriteString(`<pre class="directive" data-directive="` + escapeAttr(name) + `">`)
		if args := el.Attr("arguments"); args != "" {
			b.WriteString(escapeText(args))
			b.WriteString("\n")
		}
		b.WriteString(escapeText(doctree.AsText(el)))
		b.WriteString("</pre>")
	case doctree.TagSubstitutionDef:
		// Never rendered: a substitution definition has no visible
		// output of its own, only its (unresolved, see inline.go)
		// references do.
	case doctree.TagTable:
		writeTag(b, "table", "", el, headingLevel)
	case doctree.TagThead:
		renderRowGroup(b, "thead", "th", el, headingLevel)
	case doctree.TagTbody:
		renderRowGroup(b, "tbody", "td", el, headingLevel)
	case doctree.TagRow:
		// Reached only for a <tbody>-less table body (shouldn't happen
		// via the parser, which always wraps rows in thead/tbody) —
		// render as a body row.
		renderRow(b, "td", el, headingLevel)
	case doctree.TagEmphasis:
		writeTag(b, "em", "", el, headingLevel)
	case doctree.TagStrong:
		writeTag(b, "strong", "", el, headingLevel)
	case doctree.TagLiteral:
		writeTag(b, "code", "", el, headingLevel)
	case doctree.TagTitleReference:
		writeTag(b, "cite", "", el, headingLevel)
	case doctree.TagSubscript:
		writeTag(b, "sub", "", el, headingLevel)
	case doctree.TagSuperscript:
		writeTag(b, "sup", "", el, headingLevel)
	case doctree.TagAbbreviation, doctree.TagAcronym:
		writeTag(b, "abbr", "", el, headingLevel)
	case doctree.TagInline:
		attrs := ""
		if role := el.Attr("role"); role != "" {
			attrs = ` class="` + escapeAttr(role) + `"`
		}
		writeTag(b, "span", attrs, el, headingLevel)
	case doctree.TagReference:
		if uri := el.Attr("refuri"); uri != "" {
			writeTag(b, "a", ` href="`+escapeAttr(uri)+`"`, el, headingLevel)
		} else {
			renderChildren(b, el, headingLevel)
		}
	case doctree.TagSubstitutionRef:
		// Unresolved (see inline.go): render the substitution name as
		// plain text, the best available fallback with no value to
		// substitute.
		renderChildren(b, el, headingLevel)
	case doctree.TagFootnoteReference, doctree.TagCitationReference:
		if ref := el.Attr("refname"); ref != "" {
			b.WriteString(`<a href="#` + escapeAttr(ref) + `">`)
			renderChildren(b, el, headingLevel)
			b.WriteString("</a>")
		} else {
			renderChildren(b, el, headingLevel)
		}
	default:
		renderChildren(b, el, headingLevel)
	}
}

func writeTag(b *strings.Builder, tag, attrs string, el *doctree.Element, headingLevel int) {
	b.WriteString("<" + tag + attrs + ">")
	renderChildren(b, el, headingLevel)
	b.WriteString("</" + tag + ">")
}

// renderDefinitionPair renders a <field>/<definition_list_item>-shaped
// element (a name/term child followed by a body/definition child) as a
// <dt>/<dd> pair.
func renderDefinitionPair(b *strings.Builder, el *doctree.Element, nameTag, bodyTag string, headingLevel int) {
	for _, c := range el.Children {
		ce, ok := c.(*doctree.Element)
		if !ok {
			continue
		}
		switch ce.Tag {
		case nameTag:
			writeTag(b, "dt", "", ce, headingLevel)
		case bodyTag:
			writeTag(b, "dd", "", ce, headingLevel)
		}
	}
}

// renderRowGroup renders a <thead>/<tbody>'s row children, using
// cellTag ("th" or "td") for every entry — the doctree carries no
// "which tag" marker of its own, so the group's tag decides it here.
func renderRowGroup(b *strings.Builder, groupTag, cellTag string, el *doctree.Element, headingLevel int) {
	b.WriteString("<" + groupTag + ">")
	for _, c := range el.Children {
		if row, ok := c.(*doctree.Element); ok && row.Tag == doctree.TagRow {
			renderRow(b, cellTag, row, headingLevel)
		}
	}
	b.WriteString("</" + groupTag + ">")
}

func renderRow(b *strings.Builder, cellTag string, row *doctree.Element, headingLevel int) {
	b.WriteString("<tr>")
	for _, c := range row.Children {
		entry, ok := c.(*doctree.Element)
		if !ok || entry.Tag != doctree.TagEntry {
			continue
		}
		attrs := ""
		if mc := entry.Attr("morecols"); mc != "" {
			if n, err := strconv.Atoi(mc); err == nil {
				attrs += ` colspan="` + strconv.Itoa(n+1) + `"`
			}
		}
		if mr := entry.Attr("morerows"); mr != "" {
			if n, err := strconv.Atoi(mr); err == nil {
				attrs += ` rowspan="` + strconv.Itoa(n+1) + `"`
			}
		}
		writeTag(b, cellTag, attrs, entry, headingLevel)
	}
	b.WriteString("</tr>")
}

func escapeText(s string) string {
	return htmlEscaper.Replace(s)
}

func escapeAttr(s string) string {
	return htmlEscaper.Replace(s)
}

var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
)
