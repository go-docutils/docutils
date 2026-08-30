package rst

import "testing"

func TestIsUniformLine(t *testing.T) {
	cases := []struct {
		s        string
		wantChar rune
		wantOK   bool
	}{
		{"====", '=', true},
		{"----", '-', true},
		{"", 0, false},
		{"   ", 0, false},
		{"abc", 0, false},
		{"=-=", 0, false},
		{"==  ", '=', true}, // trailing spaces ignored
	}
	for _, tc := range cases {
		char, ok := isUniformLine(tc.s)
		if ok != tc.wantOK || (ok && char != tc.wantChar) {
			t.Errorf("isUniformLine(%q) = (%q, %v), want (%q, %v)", tc.s, char, ok, tc.wantChar, tc.wantOK)
		}
	}
}

func TestIsBulletLine(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"- item", true},
		{"-", true},
		{"-item", false}, // no space after marker, not end of line either
		{"", false},
		{"* item", true},
		{"a item", false},
	}
	for _, tc := range cases {
		if got := isBulletLine(tc.s); got != tc.want {
			t.Errorf("isBulletLine(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestIsEnumLine(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"1. item", true},
		{"1.", true},
		{"1.item", false},
		{"a. item", false},
		{"", false},
		{"12. item", true},
	}
	for _, tc := range cases {
		if got := isEnumLine(tc.s); got != tc.want {
			t.Errorf("isEnumLine(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestTrimTrailingSpace(t *testing.T) {
	cases := map[string]string{
		"abc  ": "abc",
		"abc":   "abc",
		"   ":   "",
		"":      "",
	}
	for in, want := range cases {
		if got := trimTrailingSpace(in); got != want {
			t.Errorf("trimTrailingSpace(%q) = %q, want %q", in, got, want)
		}
	}
}
