package rst

import "sort"

// A grid table's geometry is measured in the same units docutils
// measures it in, which are neither bytes nor plain code points.
//
// docutils is a Python program, so every index into a line is a CODE
// POINT index for free -- "…" occupies one column, not the three bytes
// its UTF-8 spelling takes. This package indexed the same lines as Go
// strings, i.e. as BYTES, so one "…" anywhere pushed every subsequent
// "|" two columns right of where the border said it was and the whole
// table degraded to a paragraph. Three real-world corpus files, and
// nothing partial about the loss: the entire table became text.
//
// Code points alone would be wrong in the other direction. Before
// parsing a table docutils calls StringList.pad_double_width
// (statemachine.py, from states.py's grid_table_top and
// simple_table_top), which appends a filler character after every East
// Asian Wide or Fullwidth character so it occupies TWO positions, and
// tableparser strips that filler back out of each cell
// (cellblock.replace(self.double_width_pad_char, '')). So a CJK
// character is two columns wide, and a table whose rows line up by code
// point but not by display width is one real docutils REJECTS -- which
// is verified against it here, not assumed: a "中文" cell padded to
// align by code points is a "Malformed table" error there.
//
// This file is therefore the same two-step: pad, then index by code
// point, then strip the padding out of the cell text.

// eastAsianWideRanges lists the code points unicodedata.east_asian_width
// reports as 'W' (Wide) or 'F' (Fullwidth) -- the exact test
// pad_double_width applies.
//
// GENERATED from the reference interpreter's own unicodedata, Unicode
// 16.0.0, which is the same table the corpus judge consults; see the
// README for the one-liner. Go's standard library ships no East Asian
// width data, and this package has no dependencies, so the ranges live
// here rather than behind golang.org/x/text/width.
var eastAsianWideRanges = [][2]rune{
	{0x1100, 0x115F},
	{0x231A, 0x231B},
	{0x2329, 0x232A},
	{0x23E9, 0x23EC},
	{0x23F0, 0x23F0},
	{0x23F3, 0x23F3},
	{0x25FD, 0x25FE},
	{0x2614, 0x2615},
	{0x2630, 0x2637},
	{0x2648, 0x2653},
	{0x267F, 0x267F},
	{0x268A, 0x268F},
	{0x2693, 0x2693},
	{0x26A1, 0x26A1},
	{0x26AA, 0x26AB},
	{0x26BD, 0x26BE},
	{0x26C4, 0x26C5},
	{0x26CE, 0x26CE},
	{0x26D4, 0x26D4},
	{0x26EA, 0x26EA},
	{0x26F2, 0x26F3},
	{0x26F5, 0x26F5},
	{0x26FA, 0x26FA},
	{0x26FD, 0x26FD},
	{0x2705, 0x2705},
	{0x270A, 0x270B},
	{0x2728, 0x2728},
	{0x274C, 0x274C},
	{0x274E, 0x274E},
	{0x2753, 0x2755},
	{0x2757, 0x2757},
	{0x2795, 0x2797},
	{0x27B0, 0x27B0},
	{0x27BF, 0x27BF},
	{0x2B1B, 0x2B1C},
	{0x2B50, 0x2B50},
	{0x2B55, 0x2B55},
	{0x2E80, 0x2E99},
	{0x2E9B, 0x2EF3},
	{0x2F00, 0x2FD5},
	{0x2FF0, 0x303E},
	{0x3041, 0x3096},
	{0x3099, 0x30FF},
	{0x3105, 0x312F},
	{0x3131, 0x318E},
	{0x3190, 0x31E5},
	{0x31EF, 0x321E},
	{0x3220, 0x3247},
	{0x3250, 0xA48C},
	{0xA490, 0xA4C6},
	{0xA960, 0xA97C},
	{0xAC00, 0xD7A3},
	{0xF900, 0xFAFF},
	{0xFE10, 0xFE19},
	{0xFE30, 0xFE52},
	{0xFE54, 0xFE66},
	{0xFE68, 0xFE6B},
	{0xFF01, 0xFF60},
	{0xFFE0, 0xFFE6},
	{0x16FE0, 0x16FE4},
	{0x16FF0, 0x16FF1},
	{0x17000, 0x187F7},
	{0x18800, 0x18CD5},
	{0x18CFF, 0x18D08},
	{0x1AFF0, 0x1AFF3},
	{0x1AFF5, 0x1AFFB},
	{0x1AFFD, 0x1AFFE},
	{0x1B000, 0x1B122},
	{0x1B132, 0x1B132},
	{0x1B150, 0x1B152},
	{0x1B155, 0x1B155},
	{0x1B164, 0x1B167},
	{0x1B170, 0x1B2FB},
	{0x1D300, 0x1D356},
	{0x1D360, 0x1D376},
	{0x1F004, 0x1F004},
	{0x1F0CF, 0x1F0CF},
	{0x1F18E, 0x1F18E},
	{0x1F191, 0x1F19A},
	{0x1F200, 0x1F202},
	{0x1F210, 0x1F23B},
	{0x1F240, 0x1F248},
	{0x1F250, 0x1F251},
	{0x1F260, 0x1F265},
	{0x1F300, 0x1F320},
	{0x1F32D, 0x1F335},
	{0x1F337, 0x1F37C},
	{0x1F37E, 0x1F393},
	{0x1F3A0, 0x1F3CA},
	{0x1F3CF, 0x1F3D3},
	{0x1F3E0, 0x1F3F0},
	{0x1F3F4, 0x1F3F4},
	{0x1F3F8, 0x1F43E},
	{0x1F440, 0x1F440},
	{0x1F442, 0x1F4FC},
	{0x1F4FF, 0x1F53D},
	{0x1F54B, 0x1F54E},
	{0x1F550, 0x1F567},
	{0x1F57A, 0x1F57A},
	{0x1F595, 0x1F596},
	{0x1F5A4, 0x1F5A4},
	{0x1F5FB, 0x1F64F},
	{0x1F680, 0x1F6C5},
	{0x1F6CC, 0x1F6CC},
	{0x1F6D0, 0x1F6D2},
	{0x1F6D5, 0x1F6D7},
	{0x1F6DC, 0x1F6DF},
	{0x1F6EB, 0x1F6EC},
	{0x1F6F4, 0x1F6FC},
	{0x1F7E0, 0x1F7EB},
	{0x1F7F0, 0x1F7F0},
	{0x1F90C, 0x1F93A},
	{0x1F93C, 0x1F945},
	{0x1F947, 0x1F9FF},
	{0x1FA70, 0x1FA7C},
	{0x1FA80, 0x1FA89},
	{0x1FA8F, 0x1FAC6},
	{0x1FACE, 0x1FADC},
	{0x1FADF, 0x1FAE9},
	{0x1FAF0, 0x1FAF8},
	{0x20000, 0x2FFFD},
	{0x30000, 0x3FFFD},
}

// isEastAsianWide reports whether r is Wide or Fullwidth, and so
// occupies two columns of a grid table.
func isEastAsianWide(r rune) bool {
	i := sort.Search(len(eastAsianWideRanges), func(i int) bool {
		return eastAsianWideRanges[i][1] >= r
	})
	return i < len(eastAsianWideRanges) && r >= eastAsianWideRanges[i][0]
}

// doubleWidthPad is docutils' own TableParser.double_width_pad_char: the
// filler standing in for a wide character's second column, removed again
// once a cell's text is extracted.
const doubleWidthPad = '\x00'

// padDoubleWidth is docutils' StringList.pad_double_width for one line.
func padDoubleWidth(s string) string {
	wide := false
	for _, r := range s {
		if isEastAsianWide(r) {
			wide = true
			break
		}
	}
	if !wide {
		return s
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		out = append(out, r)
		if isEastAsianWide(r) {
			out = append(out, doubleWidthPad)
		}
	}
	return string(out)
}

// TableColumnWidth returns how many columns of a reST table s occupies:
// one per code point, two for each East Asian Wide or Fullwidth
// character. It is the metric this package's own grid-table geometry
// uses, and the only metric under which a table's borders and its rows
// agree.
//
// It is exported because a WRITER needs the identical rule and had
// gotten it wrong in the mirror image: go-richdoc/rst padded its cells
// by rune count, so a CJK cell overflowed its column and a table this
// package had just parsed did not survive being written back out. Two
// implementations of one rule is what put the byte/code-point version
// of this bug here in the first place, so there is one, here, and the
// consumer calls it.
func TableColumnWidth(s string) int {
	n := 0
	for _, r := range s {
		n++
		if isEastAsianWide(r) {
			n++
		}
	}
	return n
}
