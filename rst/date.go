package rst

import (
	"strconv"
	"strings"
	"time"
)

// strftimeFormat renders t through a C strftime(3) format string, which
// is what the "date" directive's own content is (misc.Date.run, read
// directly, hands its content straight to Python's time.strftime).
//
// The conversions below are the C89/POSIX set Python documents as
// portable, plus the widely-available extensions (%e %F %T %D %R %C %G
// %g %h %k %l %n %r %s %t %u %V), each checked against this machine's
// own strftime for a fixed timestamp -- see TestStrftimeAgainstC.
//
// An UNKNOWN conversion is kept verbatim, "%" and all. Real docutils
// delegates to the platform here and the platforms disagree: this
// machine's strftime swallows the "%" and emits the bare letter, and the
// standard leaves the case undefined. Keeping both characters is the one
// choice that loses nothing -- a typo shows up in the output instead of
// silently becoming ordinary text.
func strftimeFormat(format string, t time.Time) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			b.WriteByte(format[i])
			continue
		}
		if i+1 >= len(format) {
			// A trailing "%" with nothing to convert is itself.
			b.WriteByte('%')
			break
		}
		i++
		if s, ok := strftimeConversion(format[i], t); ok {
			b.WriteString(s)
		} else {
			b.WriteByte('%')
			b.WriteByte(format[i])
		}
	}
	return b.String()
}

func strftimeConversion(c byte, t time.Time) (string, bool) {
	switch c {
	case '%':
		return "%", true
	case 'n':
		return "\n", true
	case 't':
		return "\t", true

	// Names. The C locale's own abbreviations, which are Go's defaults.
	case 'a':
		return t.Format("Mon"), true
	case 'A':
		return t.Weekday().String(), true
	case 'b', 'h':
		return t.Format("Jan"), true
	case 'B':
		return t.Month().String(), true
	case 'p':
		return t.Format("PM"), true

	// Whole-date and whole-time forms.
	case 'c':
		return t.Format("Mon Jan _2 15:04:05 2006"), true
	case 'D', 'x':
		return t.Format("01/02/06"), true
	case 'F':
		return t.Format("2006-01-02"), true
	case 'r':
		return t.Format("03:04:05 PM"), true
	case 'R':
		return t.Format("15:04"), true
	case 'T', 'X':
		return t.Format("15:04:05"), true

	// Numbers.
	case 'C':
		return pad2(t.Year() / 100), true
	case 'd':
		return pad2(t.Day()), true
	case 'e':
		return spacePad2(t.Day()), true
	case 'H':
		return pad2(t.Hour()), true
	case 'I':
		return pad2(hour12(t)), true
	case 'j':
		return pad3(t.YearDay()), true
	case 'k':
		return spacePad2(t.Hour()), true
	case 'l':
		return spacePad2(hour12(t)), true
	case 'm':
		return pad2(int(t.Month())), true
	case 'M':
		return pad2(t.Minute()), true
	case 's':
		return strconv.FormatInt(t.Unix(), 10), true
	case 'S':
		return pad2(t.Second()), true
	case 'y':
		return pad2(t.Year() % 100), true
	case 'Y':
		return strconv.Itoa(t.Year()), true

	// Weeks and weekdays. %U counts weeks from the first Sunday, %W from
	// the first Monday, both with the C formula ((yday + 7 - wday) / 7,
	// yday zero-based); %V and %G are the ISO 8601 pair, where a week
	// belongs to the year holding its Thursday, so the ISO year can
	// differ from the calendar one at either end.
	case 'u':
		wd := int(t.Weekday())
		if wd == 0 {
			wd = 7
		}
		return strconv.Itoa(wd), true
	case 'w':
		return strconv.Itoa(int(t.Weekday())), true
	case 'U':
		return pad2((t.YearDay() - 1 + 7 - int(t.Weekday())) / 7), true
	case 'W':
		return pad2((t.YearDay() - 1 + 7 - (int(t.Weekday())+6)%7) / 7), true
	case 'V':
		_, week := t.ISOWeek()
		return pad2(week), true
	case 'G':
		year, _ := t.ISOWeek()
		return strconv.Itoa(year), true
	case 'g':
		year, _ := t.ISOWeek()
		return pad2(year % 100), true

	// Zone.
	case 'z':
		return t.Format("-0700"), true
	case 'Z':
		return t.Format("MST"), true
	}
	return "", false
}

func hour12(t time.Time) int {
	h := t.Hour() % 12
	if h == 0 {
		h = 12
	}
	return h
}

func pad2(n int) string {
	if n < 10 && n >= 0 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func pad3(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}

func spacePad2(n int) string {
	if n < 10 && n >= 0 {
		return " " + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// dateDirectiveText implements misc.Date.run's own body: the format
// string is the directive's CONTENT (the directive declares no arguments
// at all, so same-line text folds into content), defaulting to
// "%Y-%m-%d" when there is none.
func dateDirectiveText(content []string) string {
	format := strings.Join(content, "\n")
	if format == "" {
		format = "%Y-%m-%d"
	}
	return strftimeFormat(format, timeNow())
}

// timeNow is a variable so a test can pin the clock; misc.Date.run reads
// the wall clock directly.
var timeNow = time.Now
