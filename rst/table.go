package rst

import (
	"strconv"
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// Simple tables, modeled on docutils' SimpleTableParser and the
// Body.simple_table_top/table/isolate_simple_table driving code
// (tableparser.py + states.py), read directly as bibliography:
//
//	=====  =====
//	col 1  col 2
//	=====  =====
//	1      Second column of row 1.
//	2      Second column of row 2.
//	4 is a span
//	------------
//	5
//	=====  =====
//
// Top/bottom borders and an optional single head/body separator are
// rows of "=" groups separated by spaces; a row of "-" groups
// underneath a row means that row's columns are merged into however
// many top-border columns the "-" groups span (colspan). Cell content
// is parsed as full body content via parseBlockLines (a cell can
// contain a nested list, a literal block, etc.), matching docutils.
//
// SCOPE (v1): GRID tables ("+---+---+" borders) are NOT implemented —
// see [[go-docutils-org]] for why simple tables were done first (grid
// tables need a real 2D cell-boundary scan in 4 directions, meaningfully
// more work). Column-margin violations (non-whitespace text outside a
// cell's column range) are never detected/reported — docutils raises a
// TableMarkupError; this parser just doesn't look. Overflow of the
// LAST column (text past its right edge, a legitimate docutils feature
// for a column intentionally left "unbounded") IS handled, matching
// docutils' check_columns. No colspec/tgroup/column-width metadata is
// produced — just <table>[<thead>]<tbody>, each holding <row><entry>.

type tableColumn struct{ start, end int }

// isSimpleTableTopLine is docutils' simple_table_top_pat: 2+ groups of
// "=" separated by spaces.
func isSimpleTableTopLine(s string) bool {
	cols := parseColumnsChar(s, '=', nil, 0)
	return len(cols) >= 2
}

// isSimpleTableBorderLine is docutils' simple_table_border_pat /
// head_body_separator_pat: starts with "=", contains only "=" and " ".
func isSimpleTableBorderLine(s string) bool {
	if s == "" || s[0] != '=' {
		return false
	}
	for _, r := range s {
		if r != '=' && r != ' ' {
			return false
		}
	}
	return true
}

func isSpanLine(s string) bool {
	if s == "" || s[0] != '-' {
		return false
	}
	for _, r := range s {
		if r != '-' && r != ' ' {
			return false
		}
	}
	return true
}

// parseColumnsChar finds the (start,end) column boundaries described by
// runs of `ch` in `line`: each run starts at the next `ch` and ends at
// the next space after it (or end of line). When `canonical` is
// non-nil (parsing a span/head-sep line against the table's own
// top-border columns), the last column is extended to the canonical
// last column's end, matching docutils' "unbounded rightmost column"
// allowance — canonical is nil only for the initial top-border parse.
func parseColumnsChar(line string, ch byte, canonical []tableColumn, borderEnd int) []tableColumn {
	var cols []tableColumn
	end := 0
	for {
		begin := strings.IndexByte(line[end:], ch)
		if begin < 0 {
			break
		}
		begin += end
		if sp := strings.IndexByte(line[begin:], ' '); sp < 0 {
			end = len(line)
		} else {
			end = begin + sp
		}
		cols = append(cols, tableColumn{begin, end})
	}
	if canonical != nil && len(cols) > 0 {
		if cols[len(cols)-1].end != borderEnd {
			return nil // malformed: span doesn't reach the table's right edge
		}
		cols[len(cols)-1].end = canonical[len(canonical)-1].end
	}
	return cols
}

// tryParseSimpleTable attempts to parse a simple table starting at
// lines[i]. Returns ok=false (no lines consumed) if lines[i] isn't a
// valid top border, or the table turns out malformed once isolated —
// in both cases the caller falls back to ordinary block parsing.
func (p *parser) tryParseSimpleTable(lines []string, i int) (*doctree.Element, int, bool) {
	if !isSimpleTableTopLine(lines[i]) {
		return nil, 0, false
	}

	// isolate_simple_table: collect through the 2nd border-shaped line
	// (or the 1st, if the table has no header), stopping at a blank
	// line or end of input.
	toplen := len(strings.TrimRight(lines[i], " "))
	found := 0
	end := -1
	for j := i + 1; j < len(lines); j++ {
		if isSimpleTableBorderLine(lines[j]) {
			if len(strings.TrimRight(lines[j], " ")) != toplen {
				return nil, 0, false // bottom/header rule doesn't match top border width
			}
			found++
			end = j
			atEnd := j == len(lines)-1
			nextBlank := !atEnd && isBlankStr(lines[j+1])
			if found == 2 || atEnd || nextBlank {
				break
			}
		}
	}
	if end < 0 {
		return nil, 0, false
	}
	block := append([]string(nil), lines[i:end+1]...)
	next := end + 1

	table, ok := p.buildSimpleTable(block)
	if !ok {
		return nil, next, false
	}
	return table, next, true
}

type simpleTableCell struct {
	morecols   int
	lineOffset int
	lines      []string
}

// buildSimpleTable runs the SimpleTableParser algorithm over an
// already-isolated block (top border ... bottom border, inclusive) and
// builds the doctree <table>.
func (p *parser) buildSimpleTable(block []string) (*doctree.Element, bool) {
	if len(block) < 3 {
		return nil, false
	}
	// setup(): top and bottom borders use '-' from here on, like span lines.
	block[0] = strings.ReplaceAll(block[0], "=", "-")
	block[len(block)-1] = strings.ReplaceAll(block[len(block)-1], "=", "-")

	// find_head_body_sep(): the one other "=" border line, if any.
	headBodySep := -1
	for k := 1; k < len(block)-1; k++ {
		if isSimpleTableBorderLine(block[k]) {
			if headBodySep >= 0 {
				return nil, false // multiple head/body separators
			}
			headBodySep = k
			block[k] = strings.ReplaceAll(block[k], "=", "-")
		}
	}

	columns := parseColumnsChar(block[0], '-', nil, 0)
	if len(columns) == 0 {
		return nil, false
	}
	borderEnd := columns[len(columns)-1].end
	firstStart, firstEnd := columns[0].start, columns[0].end

	var rows []([]simpleTableCell)
	appendRow := func(rowLines []string, start int, spanLine string, hasSpan bool) bool {
		if len(rowLines) == 0 && !hasSpan {
			return true
		}
		rowCols := columns
		if hasSpan {
			rowCols = parseColumnsChar(spanLine, '-', columns, borderEnd)
			if rowCols == nil {
				return false
			}
		}
		cells, ok := initSimpleTableRow(rowCols, columns, start)
		if !ok {
			return false
		}
		extendLastColumnForOverflow(rowLines, rowCols, &columns, &borderEnd)
		for ci, col := range rowCols {
			var cellLines []string
			for _, l := range rowLines {
				cellLines = append(cellLines, sliceColumn(l, col.start, col.end))
			}
			// A cell's text is centered or otherwise inset within its
			// fixed-width column (e.g. "  A  " in a 5-wide column) as
			// often as not — sliceColumn preserves that raw leading
			// whitespace verbatim, which parseBlockLines would otherwise
			// read as a real indent and wrap in a spurious block_quote.
			// Real docutils' own SimpleTableParser doesn't special-case
			// this either; the dedent happens implicitly because nested
			// parsing works from each cell's own coordinate system, not
			// the table's — dedentCellLines (already used by the grid-
			// table path, gridtable.go) reproduces that here directly.
			cells[ci].lines = dedentCellLines(cellLines)
		}
		rows = append(rows, cells)
		return true
	}

	offset := 1
	start := 1
	textFound := false
	for offset < len(block) {
		line := block[offset]
		switch {
		case isSpanLine(line):
			if !appendRow(block[start:offset], start, line, true) {
				return nil, false
			}
			start = offset + 1
			textFound = false
		case strings.TrimSpace(sliceColumn(line, firstStart, firstEnd)) != "":
			if textFound && offset != start {
				if !appendRow(block[start:offset], start, "", false) {
					return nil, false
				}
			}
			start = offset
			textFound = true
		case !textFound:
			start = offset + 1
		}
		offset++
	}

	table := doctree.NewElement(doctree.TagTable)
	tgroup := newTgroup(columns)
	var thead *doctree.Element
	tbody := doctree.NewElement(doctree.TagTbody)
	for _, row := range rows {
		rowEl := doctree.NewElement(doctree.TagRow)
		for _, cell := range row {
			entry := doctree.NewElement(doctree.TagEntry)
			if cell.morecols > 0 {
				entry.SetAttr("morecols", strconv.Itoa(cell.morecols))
			}
			p.parseBlockLines(trimTrailingBlankLines(cell.lines), entry, -1)
			rowEl.Append(entry)
		}
		if headBodySep >= 0 && row[0].lineOffset < headBodySep {
			if thead == nil {
				thead = doctree.NewElement(doctree.TagThead)
			}
			thead.Append(rowEl)
		} else {
			tbody.Append(rowEl)
		}
	}
	if thead != nil {
		tgroup.Append(thead)
	}
	tgroup.Append(tbody)
	table.Append(tgroup)
	return table, true
}

// newTgroup builds the <tgroup cols="N"> wrapper docutils always puts
// between <table> and its <thead>/<tbody>, with a <colspec colwidth="W">
// per column (W = that column's character width in the source table,
// verified against real docutils: the exact dash/equals-run length between
// border markers, not some normalized fraction). Writers that don't use
// this metadata (this project's html/latex, and go-richdoc/rst) fall
// through to it harmlessly — colspec has no children, and an unrecognized
// tag's default handling is to render its children (none) and nothing
// else.
func newTgroup(columns []tableColumn) *doctree.Element {
	tgroup := doctree.NewElement(doctree.TagTgroup)
	tgroup.SetAttr("cols", strconv.Itoa(len(columns)))
	for _, col := range columns {
		spec := doctree.NewElement(doctree.TagColspec)
		spec.SetAttr("colwidth", strconv.Itoa(col.end-col.start))
		tgroup.Append(spec)
	}
	return tgroup
}

// initSimpleTableRow computes each cell's morecols (colspan-1) by
// walking rowCols against the table's canonical top-border columns,
// docutils' TableParser.init_row.
func initSimpleTableRow(rowCols, canonical []tableColumn, lineOffset int) ([]simpleTableCell, bool) {
	cells := make([]simpleTableCell, 0, len(rowCols))
	ci := 0
	for _, col := range rowCols {
		if ci >= len(canonical) || col.start != canonical[ci].start {
			return nil, false
		}
		morecols := 0
		for col.end != canonical[ci].end {
			ci++
			morecols++
			if ci >= len(canonical) {
				return nil, false
			}
		}
		cells = append(cells, simpleTableCell{morecols: morecols, lineOffset: lineOffset})
		ci++
	}
	return cells, true
}

// extendLastColumnForOverflow implements the one part of docutils'
// check_columns this parser keeps: text past the last column's right
// edge enlarges that column (both for this row and canonically) rather
// than being treated as a margin error.
func extendLastColumnForOverflow(rowLines []string, rowCols []tableColumn, canonical *[]tableColumn, borderEnd *int) {
	if len(rowCols) == 0 {
		return
	}
	last := len(rowCols) - 1
	maxEnd := rowCols[last].end
	for _, l := range rowLines {
		if len(l) > maxEnd {
			text := strings.TrimRight(l, " ")
			if len(text) > maxEnd {
				maxEnd = len(text)
			}
		}
	}
	if maxEnd > rowCols[last].end {
		rowCols[last].end = maxEnd
		canLast := len(*canonical) - 1
		if maxEnd > (*canonical)[canLast].end {
			(*canonical)[canLast].end = maxEnd
			*borderEnd = maxEnd
		}
	}
}

func sliceColumn(line string, start, end int) string {
	if start >= len(line) {
		return ""
	}
	if end > len(line) {
		end = len(line)
	}
	return line[start:end]
}

func trimTrailingBlankLines(lines []string) []string {
	for len(lines) > 0 && isBlankStr(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	return lines
}
