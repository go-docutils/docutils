package rst

import (
	"strings"
	"testing"
	"time"

	"github.com/go-docutils/docutils/doctree"
)

// TestStrftimeConversions pins every conversion strftimeFormat claims
// against real C strftime, through Python's time.strftime -- the same
// call misc.Date.run makes. Each expectation below is that function's
// literal output for the timestamp named beside it, run under
// TZ=Europe/Paris and transcribed; nothing here was derived from
// strftimeFormat itself.
//
// The three timestamps are chosen to separate conversions that agree on
// an ordinary date: a summer afternoon (two-digit everything, +0200), a
// midnight on New Year's Day (zero-padding, hour 0 rendering as 12 for
// the 12-hour forms, week 00), and 2006-01-01 -- a Sunday whose ISO week
// belongs to the PREVIOUS year, so %G/%g/%V part company with %Y/%y/%U.
func TestStrftimeConversions(t *testing.T) {
	cest := time.FixedZone("CEST", 2*3600)
	cet := time.FixedZone("CET", 3600)
	cases := []struct {
		name string
		when time.Time
		want map[string]string
	}{
		{
			"a summer afternoon",
			time.Date(2026, 9, 10, 19, 14, 29, 0, cest),
			map[string]string{
				"%a": "Thu", "%A": "Thursday", "%b": "Sep", "%B": "September",
				"%c": "Thu Sep 10 19:14:29 2026", "%C": "20", "%d": "10",
				"%D": "09/10/26", "%e": "10", "%F": "2026-09-10", "%g": "26",
				"%G": "2026", "%h": "Sep", "%H": "19", "%I": "07", "%j": "253",
				"%k": "19", "%l": " 7", "%m": "09", "%M": "14", "%p": "PM",
				"%r": "07:14:29 PM", "%R": "19:14", "%s": "1789060469",
				"%S": "29", "%T": "19:14:29", "%u": "4", "%U": "36",
				"%V": "37", "%w": "4", "%W": "36", "%x": "09/10/26",
				"%X": "19:14:29", "%y": "26", "%Y": "2026", "%z": "+0200",
				"%Z": "CEST",
			},
		},
		{
			"midnight on New Year's Day",
			time.Date(2026, 1, 1, 0, 4, 5, 0, cet),
			map[string]string{
				"%a": "Thu", "%A": "Thursday", "%b": "Jan", "%B": "January",
				"%c": "Thu Jan  1 00:04:05 2026", "%C": "20", "%d": "01",
				"%D": "01/01/26", "%e": " 1", "%F": "2026-01-01", "%g": "26",
				"%G": "2026", "%h": "Jan", "%H": "00", "%I": "12", "%j": "001",
				"%k": " 0", "%l": "12", "%m": "01", "%M": "04", "%p": "AM",
				"%r": "12:04:05 AM", "%R": "00:04", "%s": "1767222245",
				"%S": "05", "%T": "00:04:05", "%u": "4", "%U": "00",
				"%V": "01", "%w": "4", "%W": "00", "%x": "01/01/26",
				"%X": "00:04:05", "%y": "26", "%Y": "2026", "%z": "+0100",
				"%Z": "CET",
			},
		},
		{
			"a January day whose ISO week belongs to the previous year",
			time.Date(2006, 1, 1, 1, 0, 0, 0, cet),
			map[string]string{
				"%a": "Sun", "%A": "Sunday", "%b": "Jan", "%B": "January",
				"%c": "Sun Jan  1 01:00:00 2006", "%C": "20", "%d": "01",
				"%D": "01/01/06", "%e": " 1", "%F": "2006-01-01", "%g": "05",
				"%G": "2005", "%h": "Jan", "%H": "01", "%I": "01", "%j": "001",
				"%k": " 1", "%l": " 1", "%m": "01", "%M": "00", "%p": "AM",
				"%r": "01:00:00 AM", "%R": "01:00", "%s": "1136073600",
				"%S": "00", "%T": "01:00:00", "%u": "7", "%U": "01",
				"%V": "52", "%w": "0", "%W": "00", "%x": "01/01/06",
				"%X": "01:00:00", "%y": "06", "%Y": "2006", "%z": "+0100",
				"%Z": "CET",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for format, want := range tc.want {
				if got := strftimeFormat(format, tc.when); got != want {
					t.Errorf("strftimeFormat(%q, %s) = %q, want %q",
						format, tc.when.Format(time.RFC3339), got, want)
				}
			}
		})
	}
}

// TestStrftimeLiteralsAndUnknowns covers the three cases that are not a
// conversion at all. "%%" and a trailing "%" match C strftime exactly.
// An UNKNOWN conversion deliberately does NOT: this machine's strftime
// swallows the "%" and emits the bare letter ("%q" -> "q"), which the
// standard leaves undefined and other platforms do differently, so the
// text is kept whole instead -- see strftimeFormat's own comment.
func TestStrftimeLiteralsAndUnknowns(t *testing.T) {
	when := time.Date(2026, 9, 10, 19, 14, 29, 0, time.FixedZone("CEST", 2*3600))
	cases := map[string]string{
		"%%":            "%",
		"abc%":          "abc%",
		"%Y%%%m":        "2026%09",
		"no conversion": "no conversion",
		"%q":            "%q",
		"100%":          "100%",
	}
	for format, want := range cases {
		if got := strftimeFormat(format, when); got != want {
			t.Errorf("strftimeFormat(%q) = %q, want %q", format, got, want)
		}
	}
}

// TestDateDirective covers misc.Date: the format string is the
// directive's CONTENT, not an argument (the directive declares none, so
// same-line text folds into content), it defaults to "%Y-%m-%d", and a
// format with no conversion in it comes back verbatim -- the corpus
// fixture's own German "täglich".
func TestDateDirective(t *testing.T) {
	fixed := time.Date(2026, 9, 10, 19, 14, 29, 0, time.FixedZone("CEST", 2*3600))
	restore := timeNow
	timeNow = func() time.Time { return fixed }
	defer func() { timeNow = restore }()

	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"a format with no conversions is its own output",
			".. |date| date:: täglich\n",
			"<document>\n    <substitution_definition name=\"date\">\n        täglich\n",
		},
		{
			"no content at all defaults to %Y-%m-%d",
			".. |date| date::\n",
			"<document>\n    <substitution_definition name=\"date\">\n        2026-09-10\n",
		},
		{
			"same-line text is CONTENT, not an argument",
			".. |date| date:: %Y\n",
			"<document>\n    <substitution_definition name=\"date\">\n        2026\n",
		},
		{
			"content starting on the following line works too",
			".. |date| date::\n\n   %d.%m.%Y\n",
			"<document>\n    <substitution_definition name=\"date\">\n        10.09.2026\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := doctree.Dump(Parse(tc.source)); got != tc.want {
				t.Errorf("Parse(%q) dump =\n%s\nwant:\n%s", tc.source, got, tc.want)
			}
		})
	}
}

// TestDateOutsideASubstitutionIsAnError pins misc.Date.run's own opening
// guard: the directive refuses any state that is not a SubstitutionDef.
// "replace" is the only other directive of that shape, and the sentence
// is the same one.
func TestDateOutsideASubstitutionIsAnError(t *testing.T) {
	got := doctree.Dump(Parse(".. date:: %Y\n"))
	want := "<document>\n    <system_message level=\"3\" line=\"1\" type=\"ERROR\">\n        <paragraph>\n            Invalid context: the \"date\" directive can only be used within a substitution definition.\n        <literal_block>\n            .. date:: %Y\n"
	if got != want {
		t.Errorf("dump =\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "Unknown directive") {
		t.Errorf("a REGISTERED directive was reported as unknown:\n%s", got)
	}
}
