package rst

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// This file ports docutils.parsers.rst.directives.tables' Table base
// class plus RSTTable (".. table::") and ListTable (".. list-table::"),
// read directly — both directives share a title (the directive's own
// argument, inline-parsed), and a common option set (:class:/:name:/
// :align:/:width:/:widths:), but differ entirely in how the table BODY
// itself is built: RSTTable dispatches its content through this
// project's EXISTING simple/grid table parser (already implemented,
// see table.go/gridtable.go) and requires exactly one <table> to result;
// ListTable instead requires exactly one uniform two-level bullet list
// (rows, each a nested list of cells) and builds the <table> structure
// from scratch. Neither has this project's own per-directive option-TYPE
// validation (an invalid :align:/:width: value, a malformed CSV in the
// separate ".. csv-table::" directive — NOT implemented here at all,
// no corpus case reached it) — matches role.go's established "no
// per-directive registry" scope boundary.

// tableCommonOptions holds the options RSTTable and ListTable share.
type tableCommonOptions struct {
	classes     []string
	name        string
	align       string
	width       string
	widthsRaw   string // "", "auto", "grid", or the raw comma/space list text
	widthsList  []int
	headerRows  int
	stubColumns int
}

// parseDirectiveOptions scans body for a leading contiguous run of
// ":key: value" lines — docutils' own parse_directive_block, simplified:
// no multi-line option-value continuation, no per-option type
// validation (see role.go's registerRole, the same established scope
// boundary) — terminated by the first non-option line. Returns the
// options found (lowercased keys) and the remaining content lines, with
// any blank lines between options and content skipped.
func parseDirectiveOptions(body []string) (options map[string]string, content []string) {
	options = map[string]string{}
	i := 0
	for i < len(body) {
		key, col, ok := matchFieldMarker(body[i])
		if !ok {
			break
		}
		options[strings.ToLower(key)] = strings.TrimSpace(body[i][col:])
		i++
	}
	for i < len(body) && isBlankStr(body[i]) {
		i++
	}
	return options, body[i:]
}

func allBlank(lines []string) bool {
	for _, l := range lines {
		if !isBlankStr(l) {
			return false
		}
	}
	return true
}

// normalizeLength mirrors directives.get_measure for the ":width:"
// option (length_or_percentage_or_unitless, read directly): a number
// optionally followed by a unit or "%", with any space between the
// number and unit removed. Unlike real docutils this doesn't validate
// the unit against CSS3's known list or error on an invalid one — no
// corpus case needs that.
func normalizeLength(s string) string {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && (s[i] == '-' || s[i] == '.' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	return s[:i] + strings.TrimSpace(s[i:])
}

// parseIntList mirrors directives.positive_int_list: comma-separated if
// a comma is present, space-separated otherwise; every entry must be a
// positive integer or the whole option is invalid (ok=false).
func parseIntList(s string) (list []int, ok bool) {
	var parts []string
	if strings.Contains(s, ",") {
		parts = strings.Split(s, ",")
	} else {
		parts = strings.Fields(s)
	}
	for _, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n < 1 {
			return nil, false
		}
		list = append(list, n)
	}
	return list, len(list) > 0
}

func parseTableCommonOptions(options map[string]string) tableCommonOptions {
	var o tableCommonOptions
	if v, ok := options["class"]; ok {
		o.classes = classOption(v)
	}
	o.name = options["name"]
	if v, ok := options["align"]; ok {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "left" || v == "center" || v == "right" {
			o.align = v
		}
	}
	if v, ok := options["width"]; ok {
		o.width = normalizeLength(v)
	}
	if v, ok := options["widths"]; ok {
		v = strings.TrimSpace(v)
		switch strings.ToLower(v) {
		case "auto", "grid":
			o.widthsRaw = strings.ToLower(v)
		default:
			if list, ok := parseIntList(v); ok {
				o.widthsRaw = v
				o.widthsList = list
			}
		}
	}
	if v, ok := options["header-rows"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
			o.headerRows = n
		}
	}
	if v, ok := options["stub-columns"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
			o.stubColumns = n
		}
	}
	return o
}

// parseTableTitle mirrors Table.make_title (tables.py, read directly):
// the directive's own argument, inline-parsed — so it can contain
// markup, including a dangling one that fails (the same
// markupProblematic machinery any other inline-markup failure uses).
func (p *parser) parseTableTitle(args string, lineno int) (*doctree.Element, []*doctree.Element) {
	if args == "" {
		return nil, nil
	}
	nodes, msgs := p.parseInline(args, lineno)
	return doctree.NewElement(doctree.TagTitle, nodes...), msgs
}

func appendClass(el *doctree.Element, cls string) {
	if existing := el.Attr("class"); existing != "" {
		el.SetAttr("class", existing+" "+cls)
	} else {
		el.SetAttr("class", cls)
	}
}

// applyTableCommonOptions attaches the shared options onto an
// already-built <table> — RSTTable.run/ListTable.run, read directly.
// isRST gates the widthsList override (RSTTable overrides EXISTING
// colspec widths only for an explicit list of numbers, leaving "auto"/
// "grid" as pure class annotations over whatever the underlying simple/
// grid table markup already computed); ListTable computes its own
// widths from scratch in runListTableDirective, so this never touches
// its colspecs.
func applyTableCommonOptions(table *doctree.Element, o tableCommonOptions, isRST bool) {
	for _, c := range o.classes {
		appendClass(table, c)
	}
	if o.width != "" {
		table.SetAttr("width", o.width)
	}
	if o.align != "" {
		table.SetAttr("align", o.align)
	}
	if o.name != "" {
		name := normalizeName(o.name)
		table.SetAttr("name", name)
		table.SetAttr("id", makeID(name))
	}
	if isRST && len(o.widthsList) > 0 {
		applyExplicitColWidths(table, o.widthsList)
	}
	switch o.widthsRaw {
	case "auto":
		appendClass(table, "colwidths-auto")
	case "grid":
		appendClass(table, "colwidths-given")
	default:
		if len(o.widthsList) > 0 {
			appendClass(table, "colwidths-given")
		}
	}
}

func applyExplicitColWidths(table *doctree.Element, widths []int) {
	var tgroup *doctree.Element
	for _, c := range table.Children {
		if e, ok := c.(*doctree.Element); ok && e.Tag == doctree.TagTgroup {
			tgroup = e
			break
		}
	}
	if tgroup == nil {
		return
	}
	var colspecs []*doctree.Element
	for _, c := range tgroup.Children {
		if e, ok := c.(*doctree.Element); ok && e.Tag == doctree.TagColspec {
			colspecs = append(colspecs, e)
		}
	}
	// real docutils errors here ("widths do not match number of
	// columns") on a mismatch; not corpus-tested, silently left as-is.
	if len(colspecs) != len(widths) {
		return
	}
	for idx, spec := range colspecs {
		spec.SetAttr("colwidth", strconv.Itoa(widths[idx]))
	}
}

// runTableDirective implements ".. table::" — RSTTable.run, read
// directly: dispatch the body (after stripping options) through this
// project's existing simple/grid table parser and require exactly one
// <table> to result.
func (p *parser) runTableDirective(lines []string, i, next int, args string, body []string) []doctree.Node {
	lineno := i + 1
	blockText := strings.Join(lines[i:next], "\n")
	options, content := parseDirectiveOptions(body)
	if len(content) == 0 || allBlank(content) {
		return []doctree.Node{sectionMessage("2", "WARNING",
			`Content block expected for the "table" directive; none found.`, lineno, blockText)}
	}
	opts := parseTableCommonOptions(options)
	title, titleMsgs := p.parseTableTitle(args, lineno)

	container := doctree.NewElement(doctree.TagDocument)
	p.parseBlockLines(content, container)
	table, ok := singleChildOfTag(container, doctree.TagTable)
	if !ok {
		return []doctree.Node{sectionMessage("3", "ERROR",
			`Error parsing content block for the "table" directive: exactly one table expected.`, lineno, blockText)}
	}

	applyTableCommonOptions(table, opts, true)
	if title != nil {
		table.Children = append([]doctree.Node{title}, table.Children...)
	}
	out := []doctree.Node{table}
	for _, m := range titleMsgs {
		out = append(out, m)
	}
	return out
}

func singleChildOfTag(container *doctree.Element, tag string) (*doctree.Element, bool) {
	if len(container.Children) != 1 {
		return nil, false
	}
	el, ok := container.Children[0].(*doctree.Element)
	if !ok || el.Tag != tag {
		return nil, false
	}
	return el, true
}

// runListTableDirective implements ".. list-table::" — ListTable.run +
// check_list_content + build_table_from_list (tables.py, read directly):
// the body must be exactly one bullet_list, uniformly two levels deep
// (each outer item's own single child is a bullet_list of cells, every
// row with the same number of cells) — each innermost list item's
// already-parsed children become one <entry>'s content directly.
func (p *parser) runListTableDirective(lines []string, i, next int, args string, body []string) []doctree.Node {
	lineno := i + 1
	blockText := strings.Join(lines[i:next], "\n")
	options, content := parseDirectiveOptions(body)
	if len(content) == 0 || allBlank(content) {
		return []doctree.Node{sectionMessage("3", "ERROR",
			`The "list-table" directive is empty; content required.`, lineno, blockText)}
	}
	opts := parseTableCommonOptions(options)
	title, titleMsgs := p.parseTableTitle(args, lineno)

	container := doctree.NewElement(doctree.TagDocument)
	p.parseBlockLines(content, container)
	outerList, ok := singleChildOfTag(container, doctree.TagBulletList)
	if !ok {
		return []doctree.Node{sectionMessage("3", "ERROR",
			`Error parsing content block for the "list-table" directive: exactly one bullet list expected.`, lineno, blockText)}
	}

	var rows [][]doctree.Node
	numCols := -1
	for rowIdx, item := range outerList.Children {
		itemEl, ok := item.(*doctree.Element)
		if !ok || itemEl.Tag != doctree.TagListItem {
			continue
		}
		innerList, ok := singleChildOfTag(itemEl, doctree.TagBulletList)
		if !ok {
			return []doctree.Node{sectionMessage("3", "ERROR", fmt.Sprintf(
				`Error parsing content block for the "list-table" directive: two-level bullet list expected, but row %d does not contain a second-level bullet list.`,
				rowIdx+1), lineno, blockText)}
		}
		if numCols == -1 {
			numCols = len(innerList.Children)
		} else if len(innerList.Children) != numCols {
			return []doctree.Node{sectionMessage("3", "ERROR", fmt.Sprintf(
				`Error parsing content block for the "list-table" directive: uniform two-level bullet list expected, but row %d does not contain the same number of items as row 1 (%d vs %d).`,
				rowIdx+1, len(innerList.Children), numCols), lineno, blockText)}
		}
		var rowCells []doctree.Node
		for _, cellItem := range innerList.Children {
			cellEl, ok := cellItem.(*doctree.Element)
			if !ok {
				continue
			}
			rowCells = append(rowCells, doctree.NewElement(doctree.TagEntry, cellEl.Children...))
		}
		rows = append(rows, rowCells)
	}

	// Table.get_column_widths, called from check_list_content BEFORE
	// check_table_dimensions (tables.py, read directly): an explicit
	// :widths: list that doesn't match the actual column count errors
	// immediately, ahead of the header-rows/stub-columns checks below.
	if len(opts.widthsList) > 0 && len(opts.widthsList) != numCols {
		return []doctree.Node{sectionMessage("3", "ERROR", fmt.Sprintf(
			`"list-table" widths do not match the number of columns in table (%d).`,
			numCols), lineno, blockText)}
	}

	if opts.headerRows > len(rows) {
		return []doctree.Node{sectionMessage("3", "ERROR", fmt.Sprintf(
			`%d header row(s) specified but only %d row(s) of data supplied ("list-table" directive).`,
			opts.headerRows, len(rows)), lineno, blockText)}
	}
	if len(rows) == opts.headerRows && opts.headerRows > 0 {
		return []doctree.Node{sectionMessage("3", "ERROR", fmt.Sprintf(
			`Insufficient data supplied (%d row(s)); no data remaining for table body, required by "list-table" directive.`,
			len(rows)), lineno, blockText)}
	}
	if opts.stubColumns > numCols {
		return []doctree.Node{sectionMessage("3", "ERROR", fmt.Sprintf(
			`%d stub column(s) specified but only %d columns(s) of data supplied ("list-table" directive).`,
			opts.stubColumns, numCols), lineno, blockText)}
	}
	if numCols == opts.stubColumns && opts.stubColumns > 0 {
		return []doctree.Node{sectionMessage("3", "ERROR", fmt.Sprintf(
			`Insufficient data supplied (%d columns(s)); no data remaining for table body, required by "list-table" directive.`,
			numCols), lineno, blockText)}
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
	if opts.headerRows > 0 {
		thead := doctree.NewElement(doctree.TagThead)
		for _, r := range rows[:opts.headerRows] {
			thead.Append(doctree.NewElement(doctree.TagRow, r...))
		}
		tgroup.Append(thead)
	}
	tbody := doctree.NewElement(doctree.TagTbody)
	for _, r := range rows[opts.headerRows:] {
		tbody.Append(doctree.NewElement(doctree.TagRow, r...))
	}
	tgroup.Append(tbody)
	table.Append(tgroup)

	applyTableCommonOptions(table, opts, false)
	if title != nil {
		table.Children = append([]doctree.Node{title}, table.Children...)
	}
	out := []doctree.Node{table}
	for _, m := range titleMsgs {
		out = append(out, m)
	}
	return out
}

// computeListTableColWidths mirrors Table.get_column_widths for
// ListTable specifically (tables.py, read directly): an explicit list
// of integers (already length-checked by the caller's own option
// parsing) is used as-is; unset or "auto" both split the width equally
// (100/numCols) — "auto" only differs in also getting the
// colwidths-auto class (applyTableCommonOptions), never in the numbers.
func computeListTableColWidths(numCols int, explicit []int) []int {
	if len(explicit) == numCols {
		return explicit
	}
	if numCols <= 0 {
		return nil
	}
	out := make([]int, numCols)
	for i := range out {
		out[i] = 100 / numCols
	}
	return out
}
