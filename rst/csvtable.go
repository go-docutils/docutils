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
// csvDelimiter ports directives.single_char_or_whitespace_or_unicode: a
// single character, the words "tab" and "space", or a Unicode code written
// as a decimal number, as hex with one of the six prefixes docutils
// accepts ("0x", "x", "\\x", "U+", "u", "\\u") or as an XML numeric
// entity ("&#x262E;"). Anything else is not a delimiter.
func csvDelimiter(arg string) (rune, bool) {
	switch arg {
	case "tab":
		return '\t', true
	case "space":
		return ' ', true
	}
	if r := []rune(arg); len(r) == 1 {
		return r[0], true
	}
	lower := strings.ToLower(arg)
	digits := ""
	switch {
	case isAllDigits(arg):
		n, err := strconv.Atoi(arg)
		if err != nil || n < 0 || n > 0x10FFFF {
			return 0, false
		}
		return rune(n), true
	case strings.HasPrefix(lower, "&#x") && strings.HasSuffix(arg, ";"):
		digits = arg[3 : len(arg)-1]
	case strings.HasPrefix(lower, "0x"):
		digits = arg[2:]
	case strings.HasPrefix(lower, "u+"):
		digits = arg[2:]
	case strings.HasPrefix(arg, "\\x"), strings.HasPrefix(arg, "\\u"):
		digits = arg[2:]
	case strings.HasPrefix(lower, "x"), strings.HasPrefix(lower, "u"):
		digits = arg[1:]
	default:
		return 0, false
	}
	n, err := strconv.ParseInt(digits, 16, 32)
	if err != nil || n < 0 || n > 0x10FFFF {
		return 0, false
	}
	return rune(n), true
}

func (p *parser) runCSVTableDirective(lines []string, i, next, lineBase int, args string, body []string) ([]doctree.Node, bool) {
	lineno := msgLine(i, lineBase)
	blockText := strings.Join(lines[i:next], "\n")
	options, content := parseDirectiveOptions(body)
	// ":delim:" and ":keepspace:" ARE supported now -- both map onto
	// encoding/csv's own reader (Comma, TrimLeadingSpace), which is where
	// this list came from: the options left in it need a configurable
	// quote or escape character, which that reader has no field for, or
	// file I/O this package does not do. sphinx's own latex.rst writes a
	// table with ":delim: ;" inside a list item, and refusing the whole
	// directive turned it into a <directive> node holding its own source.
	for _, unsupported := range []string{"file", "url", "quote", "escape", "encoding"} {
		if _, ok := options[unsupported]; ok {
			return nil, false
		}
	}
	if len(content) == 0 || allBlank(content) {
		return []doctree.Node{sectionMessage("3", "ERROR",
			`The "csv-table" directive requires content; none supplied.`, lineno, blockText)}, true
	}

	// single_char_or_whitespace_or_unicode: a single character, the words
	// "tab" and "space", or a Unicode code written decimal, hex ("0x",
	// "x", "\x", "U+", "u", "\u") or as an XML entity ("&#x262E;").
	delim, delimOK := ',', true
	if v, ok := options["delim"]; ok {
		delim, delimOK = csvDelimiter(v)
	}
	if !delimOK {
		// An unusable delimiter joins the unsupported options above rather
		// than inventing a diagnostic: docutils raises an "invalid option
		// value" error here, and this package's own scope note says
		// directive option VALUE validation is not built (see the README).
		return nil, false
	}
	readRows := func(text string) ([][]string, bool) {
		r := csv.NewReader(strings.NewReader(text))
		r.FieldsPerRecord = -1 // ragged rows are padded, not rejected
		r.Comma = delim
		// ":keepspace:" is a FLAG: present means keep the leading
		// whitespace of each field, absent means strip it.
		_, keepspace := options["keepspace"]
		r.TrimLeadingSpace = !keepspace
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

	// A diagnostic raised inside a cell reports the directive's content
	// OFFSET -- one less than the first content line's number -- plus the
	// line index WITHIN THE CELL. The ROW does not enter into it: docutils
	// hands each cell to nested_parse as its own StringList starting at
	// zero, so two cells in different rows report the same line and two
	// lines of one quoted cell report different ones. Traced on the
	// reference (wrapping Reporter.system_message to print the line it is
	// handed): 5, 5, 5, 4, 8 for a role in row one, in row two, on the
	// second line of a quoted cell, with no option block, and further
	// down the document.
	//
	// That is exactly what a lineBase of contentOffset-1 gives, so the
	// cells need no special line machinery -- only the base this used to
	// pass as -1, leaving every diagnostic in every csv table with no
	// line at all.
	cellBase := msgLine(bodyStartIndex(lines, i)+len(body)-len(content), lineBase) - 2
	cellEntry := func(text string) *doctree.Node {
		container := doctree.NewElement(doctree.TagDocument)
		// splitLines, not one line holding newlines: a QUOTED csv cell may
		// span several source lines, and docutils hands the cell to
		// nested_parse as "cell.splitlines()" -- a block. Passing the whole
		// value as a single line kept its own newlines inside the
		// paragraph's text, so a cell written
		//
		//	``compile``, ``exec``, "
		//	Detect dynamic code compilation, ...
		//	"
		//
		// -- the opening quote alone on the line, which PEP 578 does --
		// began with an EMPTY line inside the paragraph.
		p.parseBlockLines(splitLines(text), container, cellBase)
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
