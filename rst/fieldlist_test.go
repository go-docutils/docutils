package rst

import "testing"

func TestMatchFieldMarker(t *testing.T) {
	cases := []struct {
		line     string
		wantName string
		wantCol  int
		wantOK   bool
	}{
		{":author: Jane Doe", "author", 9, true},
		{":date:", "date", 6, true},
		{"", "", 0, false},
		{":", "", 0, false},
		{":: not a field", "", 0, false},
		{": not a field either", "", 0, false},
		{"not a field at all", "", 0, false},
		{":unterminated field name", "", 0, false},
	}
	for _, tc := range cases {
		name, col, ok := matchFieldMarker(tc.line)
		if ok != tc.wantOK || name != tc.wantName || (ok && col != tc.wantCol) {
			t.Errorf("matchFieldMarker(%q) = (%q, %d, %v), want (%q, %d, %v)",
				tc.line, name, col, ok, tc.wantName, tc.wantCol, tc.wantOK)
		}
	}
}

func TestIsDefinitionTermLine(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		i     int
		want  bool
	}{
		{"term followed by indented line", []string{"Term", "    body"}, 0, true},
		{"blank line breaks term detection", []string{"Term", ""}, 0, false},
		{"next line not indented", []string{"Term", "Not indented"}, 0, false},
		{"last line has no successor", []string{"Term"}, 0, false},
		{"blank line itself is never a term", []string{"", "    body"}, 0, false},
		{"indented line is never a term", []string{"  indented", "    body"}, 0, false},
		{"bullet line is never a term", []string{"- item", "    body"}, 0, false},
		{"enum line is never a term", []string{"1. item", "    body"}, 0, false},
		{"explicit markup line is never a term", []string{".. comment", "    body"}, 0, false},
		{"field marker line is never a term", []string{":field: x", "    body"}, 0, false},
		{"uniform-punctuation line is never a term", []string{"====", "    body"}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDefinitionTermLine(tc.lines, tc.i); got != tc.want {
				t.Errorf("isDefinitionTermLine(%v, %d) = %v, want %v", tc.lines, tc.i, got, tc.want)
			}
		})
	}
}
