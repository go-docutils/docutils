package rst

import (
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// promoteDocInfo is docutils' DocInfo transform (transforms/frontmatter.py),
// drastically simplified: when the document's very first top-level child is
// a field_list, registered bibliographic field names — author, authors,
// organization, address, contact, version, revision, status, date,
// copyright, dedication, abstract, matched case/whitespace-insensitively
// like any other reST name — become typed children of a single <docinfo>
// element replacing the field list in place; any other field name, or a
// recognized one whose body isn't a single paragraph, stays a plain
// <field>, folded into docinfo alongside the typed ones (this is real
// docutils' own behavior too, not a simplification — verified against the
// foreign judge: an unrecognized field is NOT left behind in a separate
// list). "dedication"/"abstract" are the two exceptions: they become
// sibling <topic class="..."> elements right after docinfo instead, each
// with a <title> naming it, again matching real docutils.
//
// SCOPE: RCS keyword substitution ($Date$, $RCSfile$, ...) is not
// implemented — a CVS/RCS-era convenience with no bearing on a modern
// pure-Go pipeline, the same category of omission as pep-reference/
// rfc-reference (see the README). A compound field body (anything but a
// single paragraph) is not promoted, matching real docutils, but this
// doesn't replicate its narrow "reparse as an escaped enumerator" fallback
// for a lone single-initial author name like "E. Xampl" being misread as
// an enumerated list — real docutils itself calls this out as a
// special-case workaround, not a rule worth porting. Only a document's OWN
// first child is checked — a title followed by a field list (the far more
// common shape in practice, since real docutils additionally hoists a
// sole top-level section's title out via a SEPARATE DocTitle transform
// this project doesn't implement, given this parser doesn't do section
// hoisting at all, see the package SCOPE note) is intentionally left
// alone: it still parses to a plain field_list nested inside the section,
// which go-richdoc/rst's own README already documents as the "leading
// field list" case it converts to Document.Meta.
func promoteDocInfo(doc *doctree.Element) {
	if len(doc.Children) == 0 {
		return
	}
	fl, ok := doc.Children[0].(*doctree.Element)
	if !ok || fl.Tag != doctree.TagFieldList {
		return
	}
	docinfo := doctree.NewElement(doctree.TagDocinfo)
	var topics []*doctree.Element
	seenTopic := map[string]bool{}
	for _, c := range fl.Children {
		field, ok := c.(*doctree.Element)
		if !ok || field.Tag != doctree.TagField {
			continue
		}
		canonical, ok := biblioFieldTag(field)
		body := fieldBodyParagraph(field)
		if !ok || body == nil {
			docinfo.Append(field)
			continue
		}
		switch canonical {
		case "dedication", "abstract":
			if seenTopic[canonical] {
				docinfo.Append(field) // a duplicate: real docutils warns, this just keeps it plain
				continue
			}
			seenTopic[canonical] = true
			topic := doctree.NewElement(doctree.TagTopic)
			topic.SetAttr("class", canonical)
			topic.Append(doctree.NewElement(doctree.TagTitle, &doctree.Text{Data: topicLabel(canonical)}))
			topic.Append(body)
			topics = append(topics, topic)
		case doctree.TagAuthors:
			docinfo.Append(authorsField(body))
		default:
			el := doctree.NewElement(canonical)
			el.Children = append(el.Children, body.Children...)
			docinfo.Append(el)
		}
	}
	var out []doctree.Node
	if len(docinfo.Children) > 0 {
		out = append(out, docinfo)
	}
	for _, t := range topics {
		out = append(out, t)
	}
	doc.Children = append(out, doc.Children[1:]...)
}

var biblioFields = map[string]string{
	"author":       doctree.TagAuthor,
	"authors":      doctree.TagAuthors,
	"organization": doctree.TagOrganization,
	"address":      doctree.TagAddress,
	"contact":      doctree.TagContact,
	"version":      doctree.TagVersion,
	"revision":     doctree.TagRevision,
	"status":       doctree.TagStatus,
	"date":         doctree.TagDate,
	"copyright":    doctree.TagCopyright,
	"dedication":   "dedication",
	"abstract":     "abstract",
}

func topicLabel(canonical string) string {
	if canonical == "dedication" {
		return "Dedication"
	}
	return "Abstract"
}

// biblioFieldTag looks up a field's name against biblioFields, normalized
// the same case/whitespace-insensitive way as any other reST name
// (normalizeName — hyperlink targets, footnote labels).
func biblioFieldTag(field *doctree.Element) (string, bool) {
	if len(field.Children) == 0 {
		return "", false
	}
	nameEl, ok := field.Children[0].(*doctree.Element)
	if !ok || nameEl.Tag != doctree.TagFieldName {
		return "", false
	}
	tag, ok := biblioFields[normalizeName(doctree.AsText(nameEl))]
	return tag, ok
}

// fieldBodyParagraph returns a biblio-eligible field's sole paragraph, or
// nil if its body isn't exactly one paragraph (docutils' own
// check_compound_biblio_field requirement).
func fieldBodyParagraph(field *doctree.Element) *doctree.Element {
	if len(field.Children) < 2 {
		return nil
	}
	bodyEl, ok := field.Children[1].(*doctree.Element)
	if !ok || bodyEl.Tag != doctree.TagFieldBody || len(bodyEl.Children) != 1 {
		return nil
	}
	para, ok := bodyEl.Children[0].(*doctree.Element)
	if !ok || para.Tag != doctree.TagParagraph {
		return nil
	}
	return para
}

// authorsField splits a plain-text author list on ";" (if that yields more
// than one name) else "," — docutils' own author_separators, tried in that
// order — into one <author> per name, dropping any inline markup the
// paragraph carried (real docutils does the same: authors_from_one_
// paragraph walks Text nodes only).
func authorsField(para *doctree.Element) *doctree.Element {
	text := doctree.AsText(para)
	names := strings.Split(text, ";")
	if len(names) == 1 {
		names = strings.Split(text, ",")
	}
	authors := doctree.NewElement(doctree.TagAuthors)
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		authors.Append(doctree.NewElement(doctree.TagAuthor, &doctree.Text{Data: n}))
	}
	return authors
}
