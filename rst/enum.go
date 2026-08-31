package rst

import "strconv"

// This file ports docutils' full enumerator recognition (states.py's
// Body.enum/parse_enumerator/is_enumerated_list_item/make_enumerator and
// EnumeratedList.enumerator, all read directly): five SEQUENCES (arabic,
// loweralpha, upperalpha, lowerroman, upperroman, plus the auto-enumerator
// "#") each in three FORMATS ("N.", "(N)", "N)"), with docutils' own
// ambiguity-resolution rules (a bare "i"/"I" defaults to roman when no
// sequence is already established; otherwise arabic/loweralpha/upperalpha/
// lowerroman/upperroman are tried in that fixed order) and its one-line
// lookahead that decides whether an enumerator-shaped line is actually a
// list item (matching a title underline, e.g., is not) — see matchEnumStart
// for a fresh list, matchEnumContinuation for a subsequent item within an
// already-established one.

// enumFormat is one of docutils' three enumerator formats.
type enumFormat struct {
	prefix, suffix string
}

var enumFormats = map[string]enumFormat{
	"period": {"", "."},
	"parens": {"(", ")"},
	"rparen": {"", ")"},
}

// enumSequenceOrder is docutils' own enum.sequences — ORDERED, tried in
// this exact order when no expected sequence is given and the text isn't
// a bare "i"/"I".
var enumSequenceOrder = []string{"arabic", "loweralpha", "upperalpha", "lowerroman", "upperroman"}

// matchEnumeratorMarker recognizes one of "N.", "(N)", "N)" at the start
// of s, where N is enumerator TEXT. Unlike docutils' own regex
// alternation (which relies on backtracking to pick the right sequence
// class for N), this locates each format's own fixed delimiter directly
// — none of N's possible shapes (digits, a single letter, a roman-charset
// run, "#") ever contain '.', '(', ')', or a space, so there's no
// ambiguity about where N ends — then validates the extracted text
// separately (isEnumeratorText). Returns the format name, the raw text,
// and how many bytes of s the whole marker consumes (not counting a
// trailing space).
func matchEnumeratorMarker(s string) (format, text string, markerLen int, ok bool) {
	if s == "" {
		return "", "", 0, false
	}
	if s[0] == '(' {
		end := indexByte(s, ')')
		if end < 2 {
			return "", "", 0, false
		}
		text = s[1:end]
		if !isEnumeratorText(text) {
			return "", "", 0, false
		}
		if !(end+1 == len(s) || s[end+1] == ' ') {
			return "", "", 0, false
		}
		return "parens", text, end + 1, true
	}
	end := indexAny(s, ".)")
	if end < 1 {
		return "", "", 0, false
	}
	text = s[:end]
	if !isEnumeratorText(text) {
		return "", "", 0, false
	}
	if !(end+1 == len(s) || s[end+1] == ' ') {
		return "", "", 0, false
	}
	if s[end] == '.' {
		return "period", text, end + 1, true
	}
	return "rparen", text, end + 1, true
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func indexAny(s, chars string) int {
	for i := 0; i < len(s); i++ {
		if indexByte(chars, s[i]) >= 0 {
			return i
		}
	}
	return -1
}

// isEnumeratorText reports whether s could be SOME sequence's enumerator
// text — the union of arabic ([0-9]+), a single letter ([a-zA-Z]), a
// same-case roman-numeral-charset run ([ivxlcdm]+ or [IVXLCDM]+), or the
// literal "#" (docutils' own sequencepats union, states.py, read
// directly). This only checks the SHAPE; which specific sequence it
// resolves to (and whether it's a semantically valid roman numeral, not
// just roman-charset letters) is classifyEnumeratorSequence's job.
func isEnumeratorText(s string) bool {
	if s == "#" {
		return true
	}
	if s == "" {
		return false
	}
	if isAllDigits(s) {
		return true
	}
	if len(s) == 1 && ((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z')) {
		return true
	}
	return isAllInSet(s, "ivxlcdm") || isAllInSet(s, "IVXLCDM")
}

func isAllInSet(s, set string) bool {
	for i := 0; i < len(s); i++ {
		if indexByte(set, s[i]) < 0 {
			return false
		}
	}
	return true
}

func sequenceMatches(sequence, text string) bool {
	switch sequence {
	case "arabic":
		return isAllDigits(text)
	case "loweralpha":
		return len(text) == 1 && text[0] >= 'a' && text[0] <= 'z'
	case "upperalpha":
		return len(text) == 1 && text[0] >= 'A' && text[0] <= 'Z'
	case "lowerroman":
		return text != "" && isAllInSet(text, "ivxlcdm")
	case "upperroman":
		return text != "" && isAllInSet(text, "IVXLCDM")
	}
	return false
}

// classifyEnumeratorSequence resolves enumerator text to a specific
// sequence and its 1-based ordinal, mirroring Body.parse_enumerator
// exactly (states.py, read directly): "#" is always the auto-enumerator;
// otherwise, if expected is given and text fits its shape, that wins
// outright (this is what lets a continuation item's "I." resolve as
// upperalpha ordinal 9 rather than upperroman, once upperalpha is
// already established); otherwise a bare "i"/"I" with NO expected
// sequence defaults to lower/upperroman specifically (so a single-
// character roman numeral can still be recognized when nothing else has
// been established yet); otherwise every sequence is tried in
// enumSequenceOrder and the first whose shape fits wins. ok is false
// only when text doesn't fit ANY sequence's shape at all (shouldn't
// happen if isEnumeratorText already passed); ordinalOK is false when
// text fits a sequence's SHAPE but isn't a semantically valid value in
// it — a malformed roman numeral like "iiii" (real docutils still
// resolves a sequence for it, just with ordinal=None).
func classifyEnumeratorSequence(text, expected string) (sequence string, ordinal int, ordinalOK, ok bool) {
	if text == "#" {
		return "#", 1, true, true
	}
	if expected != "" && sequenceMatches(expected, text) {
		sequence = expected
	} else if text == "i" {
		sequence = "lowerroman"
	} else if text == "I" {
		sequence = "upperroman"
	}
	if sequence == "" {
		for _, s := range enumSequenceOrder {
			if sequenceMatches(s, text) {
				sequence = s
				break
			}
		}
	}
	if sequence == "" {
		return "", 0, false, false
	}
	ordinal, ordinalOK = enumeratorOrdinal(sequence, text)
	return sequence, ordinal, ordinalOK, true
}

// enumeratorOrdinal converts already-classified enumerator text to its
// 1-based ordinal value within sequence, or ok=false for a shape-matching
// but semantically invalid value (Body.parse_enumerator's own
// InvalidRomanNumeralError catch, states.py, read directly).
func enumeratorOrdinal(sequence, text string) (int, bool) {
	switch sequence {
	case "arabic":
		n, err := strconv.Atoi(text)
		return n, err == nil
	case "loweralpha":
		return int(text[0]-'a') + 1, true
	case "upperalpha":
		return int(text[0]-'A') + 1, true
	case "lowerroman":
		n := make([]byte, len(text))
		for i := 0; i < len(text); i++ {
			n[i] = text[i] - 'a' + 'A'
		}
		return romanToInt(string(n))
	case "upperroman":
		return romanToInt(text)
	}
	return 0, false
}

// romanPrefixes is docutils' _ROMAN_NUMERAL_PREFIXES table, in
// descending-value order — used both to CONSTRUCT a canonical numeral
// (intToRoman, greedy subtraction) and, via romanToInt's own explicit
// group-by-group parse below, to VALIDATE one strictly.
var romanPrefixes = []struct {
	value        int
	upper, lower string
}{
	{1000, "M", "m"}, {900, "CM", "cm"}, {500, "D", "d"}, {400, "CD", "cd"},
	{100, "C", "c"}, {90, "XC", "xc"}, {50, "L", "l"}, {40, "XL", "xl"},
	{10, "X", "x"}, {9, "IX", "ix"}, {5, "V", "v"}, {4, "IV", "iv"},
	{1, "I", "i"},
}

func intToRoman(n int, upper bool) string {
	var b []byte
	for _, p := range romanPrefixes {
		s := p.upper
		if !upper {
			s = p.lower
		}
		for n >= p.value {
			n -= p.value
			b = append(b, s...)
		}
	}
	return string(b)
}

// romanToInt parses an UPPERCASE roman numeral strictly in canonical
// (well-formed) form, mirroring docutils' RomanNumeral.from_string
// exactly (states.py, read directly): M{0,4}, then a hundreds group
// (CM/CD/D?C{0,3}), a tens group (XC/XL/L?X{0,3}), and a ones group
// (IX/IV/V?I{0,3}), consuming the WHOLE input — anything left over
// (including a non-canonical-but-roman-charset string like "IIII", 4
// raw I's with no subtractive form) is invalid, not merely
// "unrecognized". Range: 1-4999 (RomanNumeral's own MIN/MAX).
func romanToInt(s string) (int, bool) {
	i, n := 0, len(s)
	result := 0
	for k := 0; k < 4 && i < n && s[i] == 'M'; k++ {
		result += 1000
		i++
	}
	if i == n {
		return finishRoman(result)
	}
	switch {
	case hasPrefixAt2(s, i, "CM"):
		result += 900
		i += 2
	case hasPrefixAt2(s, i, "CD"):
		result += 400
		i += 2
	default:
		if i < n && s[i] == 'D' {
			result += 500
			i++
		}
		for k := 0; k < 3 && i < n && s[i] == 'C'; k++ {
			result += 100
			i++
		}
	}
	if i == n {
		return finishRoman(result)
	}
	switch {
	case hasPrefixAt2(s, i, "XC"):
		result += 90
		i += 2
	case hasPrefixAt2(s, i, "XL"):
		result += 40
		i += 2
	default:
		if i < n && s[i] == 'L' {
			result += 50
			i++
		}
		for k := 0; k < 3 && i < n && s[i] == 'X'; k++ {
			result += 10
			i++
		}
	}
	if i == n {
		return finishRoman(result)
	}
	switch {
	case hasPrefixAt2(s, i, "IX"):
		result += 9
		i += 2
	case hasPrefixAt2(s, i, "IV"):
		result += 4
		i += 2
	default:
		if i < n && s[i] == 'V' {
			result += 5
			i++
		}
		for k := 0; k < 3 && i < n && s[i] == 'I'; k++ {
			result += 1
			i++
		}
	}
	if i == n {
		return finishRoman(result)
	}
	return 0, false
}

func hasPrefixAt2(s string, i int, prefix string) bool {
	return i+2 <= len(s) && s[i:i+2] == prefix
}

func finishRoman(result int) (int, bool) {
	if result < 1 || result > 4999 {
		return 0, false
	}
	return result, true
}

// makeEnumeratorMarker constructs the marker text for ordinal in
// sequence/format, plus the auto-enumerator's own marker in that same
// format — Body.make_enumerator, read directly. ok is false for an
// out-of-range ordinal (alpha > 26, roman > 4999) or an unknown format.
func makeEnumeratorMarker(ordinal int, sequence, format string) (next, auto string, ok bool) {
	fi, exists := enumFormats[format]
	if !exists {
		return "", "", false
	}
	auto = fi.prefix + "#" + fi.suffix
	var enumerator string
	switch sequence {
	case "#":
		enumerator = "#"
	case "arabic":
		enumerator = strconv.Itoa(ordinal)
	case "loweralpha", "upperalpha":
		if ordinal < 1 || ordinal > 26 {
			return "", "", false
		}
		if sequence == "upperalpha" {
			enumerator = string(rune('A' + ordinal - 1))
		} else {
			enumerator = string(rune('a' + ordinal - 1))
		}
	case "lowerroman":
		if ordinal < 1 || ordinal > 4999 {
			return "", "", false
		}
		enumerator = intToRoman(ordinal, false)
	case "upperroman":
		if ordinal < 1 || ordinal > 4999 {
			return "", "", false
		}
		enumerator = intToRoman(ordinal, true)
	default:
		return "", "", false
	}
	return fi.prefix + enumerator + fi.suffix, auto, true
}

// isEnumeratedListItem mirrors Body.is_enumerated_list_item exactly
// (states.py, read directly): valid when ordinalOK, AND the line after
// this one is blank, indented, absent (EOF), or starts with the marker
// for ordinal+1 in the same sequence/format, or the auto-enumerator. This
// single lookahead is what stops a title-looking "1. Numbered Title\n===="
// from being read as a list, AND what stops an alpha/roman run from
// continuing past the point where the NEXT ordinal's marker doesn't fit —
// letting the ambiguous line fall back to being re-tried fresh (a
// different sequence may fit it, see the enumerated_lists corpus'
// "H./I./II." case, upperalpha through H then a fresh upperroman list
// starting at I).
func isEnumeratedListItem(lines []string, i int, ordinal int, ordinalOK bool, sequence, format string) bool {
	if !ordinalOK {
		return false
	}
	if i+1 >= len(lines) {
		return true
	}
	next := lines[i+1]
	if isBlankStr(next) || leadingSpaces(next) > 0 {
		return true
	}
	nextMarker, autoMarker, ok := makeEnumeratorMarker(ordinal+1, sequence, format)
	if !ok {
		return false
	}
	return hasPrefixString(next, nextMarker) || hasPrefixString(next, autoMarker)
}

func hasPrefixString(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// matchEnumStart recognizes an enumerator marker at lines[i] AND confirms
// (via isEnumeratedListItem) it's a genuine list start, not text that
// merely looks like one — Body.enumerator's own two-step check
// (parse_enumerator with no expected sequence, then
// is_enumerated_list_item), states.py, read directly.
func matchEnumStart(lines []string, i int) (format, sequence, text string, ordinal, contentCol int, ok bool) {
	fmtName, txt, markerLen, mok := matchEnumeratorMarker(lines[i])
	if !mok {
		return "", "", "", 0, 0, false
	}
	seq, ord, ordOK, cok := classifyEnumeratorSequence(txt, "")
	if !cok || !isEnumeratedListItem(lines, i, ord, ordOK, seq, fmtName) {
		return "", "", "", 0, 0, false
	}
	col := markerLen
	for col < len(lines[i]) && lines[i][col] == ' ' {
		col++
	}
	return fmtName, seq, txt, ord, col, true
}

// matchEnumContinuation checks whether lines[i] continues an already-
// established enumerated list, mirroring EnumeratedList.enumerator
// exactly (states.py, read directly): the format must match exactly; a
// non-"#" item's sequence must match the list's own established one, the
// list must not have already switched to auto-numbering ("#"), and its
// ordinal must be EXACTLY lastOrdinal+1 (no gaps allowed within one
// list, the enumerator-sequence-validation gap this project's package
// doc comment already documents as out of scope) — "#" itself always
// passes this part, matching real docutils' own short-circuit; AND the
// same one-line lookahead as a fresh start.
func matchEnumContinuation(lines []string, i int, format, sequence string, lastOrdinal int, auto bool) (ordinal, contentCol int, newAuto, ok bool) {
	fmtName, text, markerLen, mok := matchEnumeratorMarker(lines[i])
	if !mok || fmtName != format {
		return 0, 0, auto, false
	}
	seq, ord, ordOK, cok := classifyEnumeratorSequence(text, sequence)
	if !cok {
		return 0, 0, auto, false
	}
	if seq != "#" && (seq != sequence || auto || ord != lastOrdinal+1) {
		return 0, 0, auto, false
	}
	if !isEnumeratedListItem(lines, i, ord, ordOK, seq, fmtName) {
		return 0, 0, auto, false
	}
	col := markerLen
	for col < len(lines[i]) && lines[i][col] == ' ' {
		col++
	}
	return ord, col, auto || seq == "#", true
}
