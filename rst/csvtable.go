package rst

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// runCSVTableDirective ports directives.tables.CSVTable's parse-time
// half: the content (and the :header: option, if any) is read as CSV and
// laid out with the same tgroup/colspec/thead/tbody assembly the
// list-table directive already uses.
//
// The dialect is docutils' own DocutilsDialect defaults -- comma
// delimiter, '"' quote, doubled quotes, whitespace after a delimiter
// discarded -- which is exactly Go's encoding/csv with TrimLeadingSpace.
// The options that CHANGE the dialect (:delim:, :quote:, :escape:,
// :keepspace:) are not ported, and neither are :file: and :url:, which
// read from the filesystem or the network during parsing. An invocation
// using any of those falls back to the structural <directive> capture,
// the same fallback ".. raw::" without a format takes -- see the
// README's own SCOPE note. No corpus file on either side uses one.
func (p *parser) runCSVTableDirective(lines []string, i, next, lineBase int, args string, body []string) ([]doctree.Node, bool) {
	lineno := i + 1
	blockText := strings.Join(lines[i:next], "\n")
	options, content := parseDirectiveOptions(body)
	for _, unsupported := range []string{"file", "url", "delim", "quote", "escape", "keepspace", "encoding"} {
		if _, ok := options[unsupported]; ok {
			return nil, false
		}
	}
	if len(content) == 0 || allBlank(content) {
		return []doctree.Node{sectionMessage("3", "ERROR",
			`The "csv-table" directive requires content; none supplied.`, lineno, blockText)}, true
	}

	readRows := func(text string) ([][]string, bool) {
		r := csv.NewReader(strings.NewReader(text))
		r.FieldsPerRecord = -1 // ragged rows are padded, not rejected
		r.TrimLeadingSpace = true
		recs, err := r.ReadAll()
		if err != nil {
			return nil, false
		}
		return recs, true
	}

	var headerRows [][]string
	if h, ok := options["header"]; ok && strings.TrimSpace(h) != "" {
		rows, ok := readRows(h)
		if !ok {
			return nil, false
		}
		headerRows = rows
	}
	dataRows, ok := readRows(strings.Join(content, "\n"))
	if !ok {
		return nil, false
	}

	opts := parseTableCommonOptions(options)
	// :header: rows come BEFORE any :header-rows: taken off the data.
	allRows := append(append([][]string{}, headerRows...), dataRows...)
	headerCount := len(headerRows) + opts.headerRows
	if headerCount > len(allRows) {
		return []doctree.Node{sectionMessage("3", "ERROR", fmt.Sprintf(
			`%d header row(s) specified but only %d row(s) of data supplied ("csv-table" directive).`,
			opts.headerRows, len(dataRows)), lineno, blockText)}, true
	}

	numCols := 0
	for _, r := range allRows {
		if len(r) > numCols {
			numCols = len(r)
		}
	}
	if len(opts.widthsList) > 0 && len(opts.widthsList) != numCols {
		return []doctree.Node{sectionMessage("3", "ERROR", fmt.Sprintf(
			`"csv-table" widths do not match the number of columns in table (%d).`,
			numCols), lineno, blockText)}, true
	}

	title, titleMsgs := p.parseTableTitle(args, lineno)

	cellEntry := func(text string) *doctree.Node {
		container := doctree.NewElement(doctree.TagDocument)
		p.parseBlockLines([]string{text}, container, -1)
		entry := doctree.NewElement(doctree.TagEntry, container.Children...)
		var n doctree.Node = entry
		return &n
	}
	buildRow := func(cells []string) *doctree.Element {
		row := doctree.NewElement(doctree.TagRow)
		for c := 0; c < numCols; c++ {
			text := ""
			if c < len(cells) {
				text = cells[c]
			}
			row.Append(*cellEntry(text))
		}
		return row
	}

	table := doctree.NewElement(doctree.TagTable)
	tgroup := doctree.NewElement(doctree.TagTgroup)
	tgroup.SetAttr("cols", strconv.Itoa(numCols))
	for idx, w := range computeListTableColWidths(numCols, opts.widthsList) {
		spec := doctree.NewElement(doctree.TagColspec)
		spec.SetAttr("colwidth", strconv.Itoa(w))
		if idx < opts.stubColumns {
			spec.SetAttr("stub", "1")
		}
		tgroup.Append(spec)
	}
	if headerCount > 0 {
		thead := doctree.NewElement(doctree.TagThead)
		for _, r := range allRows[:headerCount] {
			thead.Append(buildRow(r))
		}
		tgroup.Append(thead)
	}
	tbody := doctree.NewElement(doctree.TagTbody)
	for _, r := range allRows[headerCount:] {
		tbody.Append(buildRow(r))
	}
	tgroup.Append(tbody)
	table.Append(tgroup)

	p.applyTableCommonOptions(table, opts, false)
	if title != nil {
		table.Children = append([]doctree.Node{title}, table.Children...)
	}
	out := []doctree.Node{table}
	for _, m := range titleMsgs {
		out = append(out, m)
	}
	return out, true
}
