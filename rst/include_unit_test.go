// Copyright (c) the go-docutils authors.
// SPDX-License-Identifier: BSD-3-Clause

package rst

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file names unexported helpers the behavioural tests do not, which is why it
// is its own file: when a test of new internals shares a file with behavioural
// tests, stashing the source under test makes the whole test binary unbuildable and
// every assertion reports "[build failed]" instead of failing.

// TestClampIndex pins the slicing the line options inherit from Python, where a
// NEGATIVE index counts from the end and an oversized one is simply the end.
func TestClampIndex(t *testing.T) {
	cases := []struct {
		n, length, want int
	}{
		{0, 4, 0},
		{2, 4, 2},
		{4, 4, 4},
		{9, 4, 4},  // past the end is the end
		{-1, 4, 3}, // from the end
		{-4, 4, 0},
		{-9, 4, 0}, // further back than the start is the start
	}
	for _, c := range cases {
		if got := clampIndex(c.n, c.length); got != c.want {
			t.Errorf("clampIndex(%d, %d) = %d, want %d", c.n, c.length, got, c.want)
		}
	}
}

// TestInputErrorString pins the two errno messages the reference prints through
// io.error_string, and the fallback for anything else. The class name is InputError
// rather than the OS error's own because docutils wraps every OSError from its
// FileInput in one.
func TestInputErrorString(t *testing.T) {
	if got, want := inputErrorString(fs.ErrNotExist, "x.txt"),
		"InputError: [Errno 2] No such file or directory: 'x.txt'"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := inputErrorString(fs.ErrPermission, "x.txt"),
		"InputError: [Errno 13] Permission denied: 'x.txt'"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := inputErrorString(errors.New("something else"), "x.txt"); got != "InputError: something else" {
		t.Errorf("got %q", got)
	}
}

// TestRelativeToCWD pins the port of utils.relative_path(None, target): a path
// relative to the process's working directory, "/" separators, and the ABSOLUTE
// path when the two have nothing in common -- which the reference decides by
// comparing the first two components, so on a POSIX system "/tmp/..." against a
// working directory under "/Users/..." already counts as nothing in common.
func TestRelativeToCWD(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Inside the working directory: relative, with no leading "..".
	if got := relativeToCWD(filepath.Join(cwd, "sub", "file.txt")); got != "sub/file.txt" {
		t.Errorf("inside the cwd: got %q, want %q", got, "sub/file.txt")
	}
	// A sibling of the working directory: one "..".
	if got := relativeToCWD(filepath.Join(filepath.Dir(cwd), "sibling.txt")); got != "../sibling.txt" {
		t.Errorf("a sibling: got %q, want %q", got, "../sibling.txt")
	}
	// Nothing in common: absolute, unchanged.
	const nothing = "/zzz-no-such-root/file.txt"
	if got := relativeToCWD(nothing); got != nothing {
		t.Errorf("nothing in common: got %q, want %q", got, nothing)
	}
}

// TestClipIncludeNegativeLines is the behaviour TestClampIndex implies, through the
// option that uses it.
func TestClipIncludeNegativeLines(t *testing.T) {
	text := "one\ntwo\nthree\nfour\n"
	got, msg := clipInclude(text, map[string]string{"start-line": "-2"})
	if msg != "" {
		t.Fatalf("unexpected error: %s", msg)
	}
	if !strings.Contains(got, "four") || strings.Contains(got, "one") {
		t.Errorf("a negative start-line should count from the end, got %q", got)
	}
}

// TestClipIncludeEdgeCases covers the three branches the ordinary options do not:
// an end BEFORE the start, and the EMPTY forms of the two text options, which the
// reference defines as "up to the first blank line" rather than as no-ops.
func TestClipIncludeEdgeCases(t *testing.T) {
	text := "one\ntwo\n\nthree\nfour\n"
	if got, msg := clipInclude(text, map[string]string{"start-line": "3", "end-line": "1"}); msg != "" || got != "" {
		t.Errorf("an end before the start should clip to nothing, got %q %q", got, msg)
	}
	got, msg := clipInclude(text, map[string]string{"start-after": ""})
	if msg != "" {
		t.Fatalf("unexpected error: %s", msg)
	}
	if strings.Contains(got, "one") || !strings.Contains(got, "three") {
		t.Errorf("an empty start-after should skip to past the first blank line, got %q", got)
	}
	got, msg = clipInclude(text, map[string]string{"end-before": ""})
	if msg != "" {
		t.Fatalf("unexpected error: %s", msg)
	}
	if !strings.Contains(got, "one") || strings.Contains(got, "three") {
		t.Errorf("an empty end-before should stop at the first blank line, got %q", got)
	}
}

// TestIntOptionIgnoresWhatIsNotANumber keeps a malformed option from being read as
// zero, which would silently clip from the first line.
func TestIntOptionIgnoresWhatIsNotANumber(t *testing.T) {
	if _, ok := intOption(map[string]string{"start-line": "abc"}, "start-line"); ok {
		t.Error("a non-numeric option should not be read as a number")
	}
	if _, ok := intOption(map[string]string{}, "start-line"); ok {
		t.Error("a missing option should not be read as a number")
	}
	if n, ok := intOption(map[string]string{"start-line": " 3 "}, "start-line"); !ok || n != 3 {
		t.Errorf("got %d, %v; want 3, true", n, ok)
	}
}
