package rst

import (
	"sort"
	"strconv"
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// Grid tables, modeled on docutils' GridTableParser and the
// Body.grid_table_top/table/isolate_grid_table driving code
// (tableparser.py + states.py), read in full as bibliography:
//
//	+------------------------+------------+----------+----------+
//	| Header row, column 1   | Header 2   | Header 3 | Header 4 |
//	+========================+============+==========+==========+
//	| body row 1, column 1   | column 2   | column 3 | column 4 |
//	+------------------------+------------+----------+----------+
//	| body row 2             | Cells may span columns.          |
//	+------------------------+------------+---------------------+
//	| body row 3             | Cells may  | - Table cells       |
//	+------------------------+ span rows. | - contain           |
//	| body row 4             |            | - body elements.    |
//	+------------------------+------------+---------------------+
//
// Intersections use '+', row separators use '-' (except an optional
// single head/body separator, which uses '='), column separators use
// '|'. Unlike simple tables, a cell can span multiple ROWS as well as
// columns (see "body row 3"/"body row 4" above, where the middle and
// right columns' cells span two text rows).
//
// The algorithm traces cell rectangles by BFS from a queue of
// "upper-left corner" candidates, starting at the table's own
// upper-left corner: for each candidate, scan right along the top edge
// for the next '+' (noting internal column boundaries along the way),
// then down the right edge, then left along the bottom edge, then back
// up the left edge to confirm the rectangle closes — a mirror of
// scan_right/scan_down/scan_left/scan_up in tableparser.py. A
// successfully-traced cell's other two corners (its top-right and
// bottom-left) are queued as new candidates, since a cell's edge is
// where the next cell over or below begins. Once every text column is
// accounted for (`done`), the set of all row/column boundary positions
// seen defines the table's actual row/column grid, and each cell's
// morerows/morecols span is how many of those grid lines it crosses.
//
// SCOPE: same simplifications as table.go's simple tables — cell
// content is parsed as full body content via parseBlockLines (so a
// cell CAN hold a nested list, unlike the LaTeX writer's flattened
// rendering of it), but column-margin/malformed-table diagnostics are
// never reported: a table that doesn't parse cleanly (an incomplete
// rectangle, an unclosed cell, a width mismatch) is simply not
// recognized as a table at all, falling back to ordinary block parsing
// of the ambiguous lines rather than emitting an error node.

func isGridTableEdgeChar(r rune) bool { return r == '+' || r == '|' }

// isGridTableTopLine is docutils' grid_table_top_pat: a full border row
// made only of '+' and '-', at least 4 columns wide, starting and
// ending with '+' and with '-' immediately inside each end (ruling out
// a degenerate "++" or "+-+" some other check might otherwise accept).
func isGridTableTopLine(s string) bool {
	t := strings.TrimSpace(s)
	if len(t) < 4 || t[0] != '+' || t[len(t)-1] != '+' || t[1] != '-' || t[len(t)-2] != '-' {
		return false
	}
	for i := 1; i < len(t)-1; i++ {
		if t[i] != '+' && t[i] != '-' {
			return false
		}
	}
	return true
}

// isGridTableHeadSepLine is docutils' head_body_separator_pat: the same
// shape as the top/bottom border, but built from '=' instead of '-'.
func isGridTableHeadSepLine(s string) bool {
	t := strings.TrimSpace(s)
	if len(t) < 4 || t[0] != '+' || t[len(t)-1] != '+' || t[1] != '=' || t[len(t)-2] != '=' {
		return false
	}
	for i := 1; i < len(t)-1; i++ {
		if t[i] != '+' && t[i] != '=' {
			return false
		}
	}
	return true
}

// isolateGridTable collects the lines belonging to one grid table
// starting at lines[i] (already confirmed to be a valid top border):
// docutils' isolate_grid_table. Every line is trimmed of surrounding
// whitespace (matching docutils' block[i] = block[i].strip()) and must
// have exactly the top border's width with '+' or '|' at each end;
// scanning stops at the first line that doesn't, then searches
// backward for the last line among those collected that is itself a
// valid full border — the table's true bottom.
// isolateGridTable is docutils' isolate_grid_table. It returns a
// detail string when the lines START as a table (the caller has already
// matched a top border) but do not FORM one: docutils reports that as
// "Malformed table." plus the detail, with the whole block quoted, and
// this used to report nothing at all and let the lines fall back to
// ordinary block parsing.
//
// offset is the line WITHIN the block the message points at, which
// differs per failure and is the fiddly part -- see the two sites
// below.
func isolateGridTable(lines []string, i int) (block []string, next int, detail string, offset int, ok bool) {
	// The text block: everything up to a blank line or the end. docutils
	// takes it with get_text_block BEFORE looking at any edge, which is
	// why a line that breaks the shape shortens the block rather than
	// ending the scan.
	j := i
	for j < len(lines) && !isBlankStr(lines[j]) {
		j++
	}
	raw := make([]string, 0, j-i)
	for k := i; k < j; k++ {
		raw = append(raw, strings.TrimSpace(padDoubleWidth(lines[k])))
	}
	if len(raw) == 0 {
		return nil, 0, "", 0, false
	}
	width := len([]rune(raw[0]))

	// Left edge: the first line that does not begin with "+" or "|"
	// truncates the block. Its index is remembered because it becomes
	// the message's offset if the bottom border then turns out to be
	// missing -- docutils gets that by letting the loop variable
	// survive into the next loop, which is a Python detail this spells
	// out instead.
	leftBreak := len(raw) - 1
	for k, l := range raw {
		r := []rune(l)
		if len(r) == 0 || !isGridTableEdgeChar(r[0]) {
			leftBreak = k
			raw = raw[:k]
			break
		}
	}
	if len(raw) < 2 {
		if len(raw) == 0 {
			return nil, 0, "", 0, false
		}
		return raw, 0, "Bottom border missing or corrupt.", leftBreak, false
	}

	// Bottom border: the last line must be one, or the last one that is
	// ends the table.
	if !isGridTableTopLine(raw[len(raw)-1]) {
		found := -1
		for k := len(raw) - 2; k > 1; k-- {
			if isGridTableTopLine(raw[k]) {
				found = k
				break
			}
		}
		if found < 0 {
			return raw, 0, "Bottom border missing or corrupt.", leftBreak, false
		}
		raw = raw[:found+1]
	}

	// Right edge: every line the same width -- combining characters
	// excluded, as in column_width -- and ending on "+" or "|".
	for k, l := range raw {
		r := []rune(l)
		if len(stripCombining(r)) != width || !isGridTableEdgeChar(r[len(r)-1]) {
			return raw, 0, "Right border not aligned or missing.", k, false
		}
	}
	return raw, i + len(raw), "", 0, true
}

// stripCombining drops the combining characters from r, which
// docutils' own right-edge check does before comparing widths
// (strip_combining_chars).
func stripCombining(r []rune) []rune {
	out := r[:0:0]
	for _, c := range r {
		if !isCombining(c) {
			out = append(out, c)
		}
	}
	return out
}

func (p *parser) tryParseGridTable(lines []string, i, lineBase int) ([]doctree.Node, int, bool) {
	if !isGridTableTopLine(lines[i]) {
		return nil, 0, false
	}
	block, next, detail, offset, ok := isolateGridTable(lines, i)
	if detail != "" {
		// Only the lines the block actually COVERS are consumed: the
		// rest of the text block is whatever interrupted the table and
		// goes on to be parsed as itself. docutils gets this from
		// previous_line/del block[i:]; here it is len(block).
		next = i + len(block)
		return blankLineAfterTable([]doctree.Node{malformedTable(block, detail, offset, i, lineBase)},
			lines, next, lineBase), next, true
	}
	if !ok {
		return nil, 0, false
	}
	table, ok := p.buildGridTable(block, i, lineBase)
	if !ok {
		return nil, 0, false
	}
	return blankLineAfterTable([]doctree.Node{table}, lines, next, lineBase), next, true
}

// blankLineAfterTable appends docutils' table_top warning: a table --
// well-formed or not -- that is not followed by a blank line draws
// "Blank line required after table." That is raised by the CALLER of
// the table parser there, which is why it applies to both outcomes and
// why neither path had it here.
func blankLineAfterTable(out []doctree.Node, lines []string, next, lineBase int) []doctree.Node {
	if next >= len(lines) || isBlankStr(lines[next]) {
		return out
	}
	return append(out, sectionMessage("2", "WARNING",
		"Blank line required after table.", msgLine(next, lineBase), ""))
}

type gridCell struct {
	top, left, bottom, right int
	lines                    []string
}

func (p *parser) buildGridTable(block []string, i, lineBase int) (*doctree.Element, bool) {
	n := len(block)
	if n < 3 {
		return nil, false
	}
	// Code points, not bytes: see eastasian.go for why, and for the
	// padding the block already carries by the time it gets here.
	grid := make([][]rune, n)
	width := len([]rune(block[0]))
	for i, l := range block {
		r := []rune(l)
		if len(r) != width {
			return nil, false
		}
		grid[i] = r
	}
	bottom := n - 1
	right := width - 1

	headBodySepRow := -1
	for i := 1; i < n-1; i++ {
		if isGridTableHeadSepLine(string(grid[i])) {
			if headBodySepRow >= 0 {
				return nil, false // multiple head/body separators
			}
			headBodySepRow = i
			for c := range grid[i] {
				if grid[i][c] == '=' {
					grid[i][c] = '-'
				}
			}
		}
	}

	done := make([]int, width)
	for i := range done {
		done[i] = -1
	}
	var cells []gridCell
	rowseps := map[int]bool{0: true}
	colseps := map[int]bool{0: true}

	type corner struct{ top, left int }
	queue := []corner{{0, 0}}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		top, left := c.top, c.left
		if top == bottom || left == right || top <= done[left] {
			continue
		}
		cbottom, cright, crowseps, ccolseps, ok := scanCell(grid, top, left, bottom, right)
		if !ok {
			continue
		}
		for k := range crowseps {
			rowseps[k] = true
		}
		for k := range ccolseps {
			colseps[k] = true
		}
		for col := left; col < cright; col++ {
			done[col] = cbottom - 1
		}
		cells = append(cells, gridCell{top, left, cbottom, cright, extractGridCellBlock(block, top, left, cbottom, cright)})
		queue = append(queue, corner{top, cright}, corner{cbottom, left})
		sort.Slice(queue, func(a, b int) bool {
			if queue[a].top != queue[b].top {
				return queue[a].top < queue[b].top
			}
			return queue[a].left < queue[b].left
		})
	}

	last := bottom - 1
	for col := 0; col < right; col++ {
		if done[col] != last {
			return nil, false // malformed: parse incomplete
		}
	}

	return p.gridTableFromCells(cells, rowseps, colseps, headBodySepRow, i, lineBase), true
}

// scanCell traces one cell rectangle starting at its upper-left corner
// (top,left), which must be '+'. Mirrors scan_right/scan_down/
// scan_left/scan_up combined into one pass per direction.
func scanCell(grid [][]rune, top, left, bottom, right int) (bottomR, rightR int, rowseps, colseps map[int]bool, ok bool) {
	colseps = map[int]bool{}
	line := grid[top]
	for i := left + 1; i <= right; i++ {
		switch line[i] {
		case '+':
			colseps[i] = true
			if br, rs, cs2, ok2 := scanDown(grid, top, left, i, bottom); ok2 {
				for k := range cs2 {
					colseps[k] = true
				}
				return br, i, rs, colseps, true
			}
		case '-':
			// continue scanning right
		default:
			return 0, 0, nil, nil, false
		}
	}
	return 0, 0, nil, nil, false
}

func scanDown(grid [][]rune, top, left, right, bottom int) (bottomR int, rowseps, colseps map[int]bool, ok bool) {
	rowseps = map[int]bool{}
	for i := top + 1; i <= bottom; i++ {
		switch grid[i][right] {
		case '+':
			rowseps[i] = true
			if rs2, cs, ok2 := scanLeft(grid, top, left, i, right); ok2 {
				for k := range rs2 {
					rowseps[k] = true
				}
				return i, rowseps, cs, true
			}
		case '|':
			// continue scanning down
		default:
			return 0, nil, nil, false
		}
	}
	return 0, nil, nil, false
}

func scanLeft(grid [][]rune, top, left, bottom, right int) (rowseps, colseps map[int]bool, ok bool) {
	colseps = map[int]bool{}
	line := grid[bottom]
	for i := right - 1; i > left; i-- {
		switch line[i] {
		case '+':
			colseps[i] = true
		case '-':
			// continue
		default:
			return nil, nil, false
		}
	}
	if line[left] != '+' {
		return nil, nil, false
	}
	rs, ok := scanUp(grid, top, left, bottom, right)
	if !ok {
		return nil, nil, false
	}
	return rs, colseps, true
}

func scanUp(grid [][]rune, top, left, bottom, right int) (rowseps map[int]bool, ok bool) {
	rowseps = map[int]bool{}
	for i := bottom - 1; i > top; i-- {
		switch grid[i][left] {
		case '+':
			rowseps[i] = true
		case '|':
			// continue
		default:
			return nil, false
		}
	}
	return rowseps, true
}

// extractGridCellBlock pulls a cell's interior text (excluding its own
// border characters): rows top+1..bottom-1, columns left+1..right-1.
// The raw slice still carries the conventional one-space padding after
// "|" and any right-edge fill used to keep every row the fixed column
// width — dedentCellLines strips both before this text is treated as
// reST body content, or that incidental padding reads as a real indent
// and produces a spurious block_quote around every cell.
func extractGridCellBlock(block []string, top, left, bottom, right int) []string {
	var out []string
	for r := top + 1; r < bottom; r++ {
		line := []rune(block[r])
		if left+1 < right && right <= len(line) {
			// The filler standing in for a wide character's second
			// column leaves with the slice, exactly as docutils'
			// cellblock.replace(double_width_pad_char, '') does.
			out = append(out, strings.ReplaceAll(string(line[left+1:right]), string(doubleWidthPad), ""))
		} else {
			out = append(out, "")
		}
	}
	return dedentCellLines(out)
}

// dedentCellLines rstrips every line (right-edge column-fill padding),
// then strips the common leading-space count shared by every non-blank
// line (the "| " left padding) — a Python-textwrap.dedent-shaped
// operation, not just a fixed one-space trim, since a cell's actual
// reST content may itself be indented relative to that shared baseline
// (a nested block quote a document author put there on purpose).
func dedentCellLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.TrimRight(l, " ")
	}
	common := -1
	for _, l := range out {
		if isBlankStr(l) {
			continue
		}
		n := leadingSpaces(l)
		if common < 0 || n < common {
			common = n
		}
	}
	if common <= 0 {
		return out
	}
	for i, l := range out {
		if len(l) >= common {
			out[i] = l[common:]
		} else {
			out[i] = ""
		}
	}
	return out
}

// gridTableFromCells is docutils' structure_from_cells: map every
// cell's raw character-grid coordinates to actual table row/column
// numbers (via the sorted set of all row/column boundaries seen), then
// build <table>[<thead>]<tbody> with each cell's morerows/morecols span.
func (p *parser) gridTableFromCells(cells []gridCell, rowseps, colseps map[int]bool, headBodySepRow, i, lineBase int) *doctree.Element {
	rowBounds := sortedIntKeys(rowseps)
	colBounds := sortedIntKeys(colseps)
	rowIndex := make(map[int]int, len(rowBounds))
	for idx, v := range rowBounds {
		rowIndex[v] = idx
	}
	colIndex := make(map[int]int, len(colBounds))
	for idx, v := range colBounds {
		colIndex[v] = idx
	}
	numRows := len(rowBounds) - 1
	numCols := len(colBounds) - 1

	type placed struct {
		morerows, morecols int
		top                int // the cell's own top border row, within block
		lines              []string
	}
	placedGrid := make([][]*placed, numRows)
	for r := range placedGrid {
		placedGrid[r] = make([]*placed, numCols)
	}
	for _, c := range cells {
		rn, cn := rowIndex[c.top], colIndex[c.left]
		placedGrid[rn][cn] = &placed{
			morerows: rowIndex[c.bottom] - rn - 1,
			morecols: colIndex[c.right] - cn - 1,
			top:      c.top,
			lines:    c.lines,
		}
	}

	headRows := 0
	if headBodySepRow >= 0 {
		headRows = rowIndex[headBodySepRow]
	}

	table := doctree.NewElement(doctree.TagTable)
	gridCols := make([]tableColumn, numCols)
	for cn := range gridCols {
		// colBounds[cn]/[cn+1] are the '+' positions bracketing this
		// column; -1 excludes the separator itself, matching newTgroup's
		// existing end-start convention for a simple table's columns.
		gridCols[cn] = tableColumn{start: colBounds[cn], end: colBounds[cn+1] - 1}
	}
	tgroup := newTgroup(gridCols)
	var thead *doctree.Element
	tbody := doctree.NewElement(doctree.TagTbody)
	for rn := 0; rn < numRows; rn++ {
		rowEl := doctree.NewElement(doctree.TagRow)
		for cn := 0; cn < numCols; cn++ {
			pc := placedGrid[rn][cn]
			if pc == nil {
				continue // covered by another cell's row/column span
			}
			entry := doctree.NewElement(doctree.TagEntry)
			if pc.morecols > 0 {
				entry.SetAttr("morecols", strconv.Itoa(pc.morecols))
			}
			if pc.morerows > 0 {
				entry.SetAttr("morerows", strconv.Itoa(pc.morerows))
			}
			// A cell's content is block rows pc.top+1..pc.bottom-1,
			// and block[r] is lines[i+r] (isolateGridTable returns the
			// block starting at i, one entry per source line), so the
			// cell's own base is i+pc.top+1+lineBase. Every cell in a
			// row gets a DIFFERENT one, which is why this could not be
			// a single per-table value and stayed at the "unknown"
			// sentinel for so long.
			p.parseBlockLines(trimTrailingBlankLines(pc.lines), entry,
				nestedLineBase(i+pc.top+1, lineBase))
			rowEl.Append(entry)
		}
		if headBodySepRow >= 0 && rn < headRows {
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
	return table
}

func sortedIntKeys(m map[int]bool) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

// malformedTable is docutils' malformed_table: an ERROR carrying
// "Malformed table." plus the detail, and the offending block quoted as
// a literal_block with the double-width padding taken back out.
//
// The line is the block's FIRST line plus the offset, which is what
// "startline = abs_line_number() - len(block) + 1" then
// "line=startline+offset" comes to.
func malformedTable(block []string, detail string, offset, i, lineBase int) *doctree.Element {
	text := strings.ReplaceAll(strings.Join(block, "\n"), string(doubleWidthPad), "")
	return sectionMessage("3", "ERROR", "Malformed table.\n"+detail,
		msgLine(i+offset, lineBase), text)
}
