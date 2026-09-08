package rst

import (
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// blockQuoteDirectiveClasses maps the three directives that are nothing
// but "a block quote carrying a class": BlockQuote.run calls
// state.block_quote() on its own content and then appends its class list
// (body.py, read directly). They declare NO options at all -- a
// ":class:" line under one is CONTENT, and parses as a field list inside
// the quote, which is what the reference does.
var blockQuoteDirectiveClasses = map[string]string{
	"epigraph":   "epigraph",
	"highlights": "highlights",
	"pull-quote": "pull-quote",
}

// runBlockQuoteDirective implements all three. The content is parsed by
// the SAME routine a bare indented block quote uses, so an attribution
// line ("-- Author") becomes an <attribution> here exactly as it would
// there.
func (p *parser) runBlockQuoteDirective(name, args string, body []string, bodyStart, lineBase int) []doctree.Node {
	class := blockQuoteDirectiveClasses[strings.ToLower(name)]

	// No options and no arguments: everything after "::" is content, so
	// same-line text joins the body the way parse_directive_block's
	// no-argument fold-back does.
	content := body
	if strings.TrimSpace(args) != "" {
		content = append([]string{args}, body...)
		bodyStart--
	}
	for len(content) > 0 && isBlankStr(content[0]) {
		content = content[1:]
		bodyStart++
	}
	for len(content) > 0 && isBlankStr(content[len(content)-1]) {
		content = content[:len(content)-1]
	}
	if len(content) == 0 {
		return []doctree.Node{sectionMessage("3", "ERROR",
			`Content block expected for the "`+name+`" directive; none found.`,
			msgLine(bodyStart-1, lineBase), "")}
	}

	quotes := p.blockQuotesFromBlock(content, bodyStart, lineBase)
	out := make([]doctree.Node, 0, len(quotes))
	for _, q := range quotes {
		if q.Tag == doctree.TagBlockQuote {
			q.SetAttr("class", class)
		}
		out = append(out, q)
	}
	return out
}
