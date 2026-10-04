// Copyright (c) the go-docutils authors.
// SPDX-License-Identifier: BSD-3-Clause

package rst

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// includeEntry is one line of the inclusion chain: the path, and the clipping
// options that go with it. Two inclusions of the same file with DIFFERENT clipping
// are not circular, which is why the options are part of the key -- Include.run's
// own `(source, self.clip_options)` tuple, read from
// docutils/parsers/rst/directives/misc.py.
type includeEntry struct {
	path string
	clip string
}

// endOfInclusionPrefix marks the end of spliced content. The reference appends a
// comment with this prefix after every inclusion and pops its log entry when the
// comment is parsed (states.py, Body.comment, which returns NO node for it), which
// is how a file included twice in sequence is allowed while a file that includes
// itself is not. Ported rather than approximated: a per-document "already seen" set
// would refuse the first of those two.
const endOfInclusionPrefix = `end of inclusion from "`

// runIncludeDirective implements the "include" directive: read a file, optionally
// clip it, and either splice its text into the input being parsed or return it as a
// literal or code block.
//
// https://docutils.sourceforge.io/docs/ref/rst/directives.html#include
//
// The splice is what makes an included file behave as if it had been typed in
// place: a section title in it nests under the including document's sections, a
// construct is not cut at the file boundary, and ids are assigned in document
// order. Parsing the file separately and grafting its nodes would get all three
// wrong, so this sets p.pendingInclude and the two block loops splice it (see
// parseDocument and parseBlockLines).
//
// What is NOT ported, deliberately: ":parser:", which runs another docutils parser
// over the content and has no meaning in this package; the "<name>" form, which
// reads from docutils' own bundled include directory (isonum.txt and friends),
// since this package bundles no such files; and the line-length limit, which this
// parser has no setting for.
func (p *parser) runIncludeDirective(lines []string, i, next, lineBase int, args string, body []string) ([]doctree.Node, bool) {
	lineno := msgLine(i, lineBase)
	blockText := strings.Join(lines[i:next], "\n")

	blanks := 0
	for j := i + 1; j < len(lines) && isBlankStr(lines[j]); j++ {
		blanks++
	}
	combined := make([]string, 0, 1+blanks+len(body))
	combined = append(combined, args)
	for k := 0; k < blanks; k++ {
		combined = append(combined, "")
	}
	combined = append(combined, body...)
	argument, options, _, _, _ := parseDirectiveBlockAt(combined, true)

	// Disabled is the FIRST check in the reference too, before the argument is
	// even looked at, and it is a WARNING rather than an error. Here it means
	// "the caller gave no SourcePath", which is the only state in which a
	// relative path cannot be resolved -- and the safe default for a parser
	// handed nothing but a string: no caller gets the filesystem read by
	// surprise.
	if p.opts.SourcePath == "" {
		if !p.opts.ReportUnknownDirectives {
			// The same answer every directive this parser cannot ACT on gets
			// when the caller has turned the reports off: fall back to the
			// structural capture, so the directive and its content survive as a
			// <directive> element for whoever is converting the document. A
			// consumer that deliberately parses without a path -- go-richdoc/rst
			// does -- would otherwise find an include replaced by a diagnostic.
			return nil, false
		}
		return []doctree.Node{sectionMessage("2", "WARNING",
			`"include" directive disabled.`, lineno, blockText)}, true
	}
	if strings.TrimSpace(argument) == "" {
		return []doctree.Node{directiveError("include", "1 argument(s) required, 0 supplied", lineno, blockText)}, true
	}
	path := strings.TrimSpace(argument)
	if strings.HasPrefix(path, "<") && strings.HasSuffix(path, ">") {
		// The bundled-file form. The reference resolves it against its own
		// parsers/rst/include directory; this package ships no such files, so
		// the honest answer is the same error a missing file gets, naming what
		// was asked for.
		return []doctree.Node{sectionMessage("3", "ERROR",
			"Problems with \"include\" directive path:\nInputError: [Errno 2] No such file or directory: '"+
				path+"'.", lineno, blockText)}, true
	}
	resolved := path
	if !filepath.IsAbs(resolved) {
		// Against the INNERMOST file's directory, not the document's. The
		// reference resolves every include against
		// `self.state.document.current_source`, which follows the inclusion as
		// its lines are spliced in -- so a file included from a subdirectory
		// includes ITS neighbours, not the top document's. The stack is pushed
		// here and popped by the end-of-inclusion marker, like the log beside it.
		base := filepath.Dir(p.opts.SourcePath)
		if n := len(p.includeDirs); n > 0 {
			base = p.includeDirs[n-1]
		}
		resolved = filepath.Join(base, resolved)
	}
	// The path as the REFERENCE spells it in a message and in its log: relative
	// to the process's working directory, with "/" separators, falling back to
	// absolute when there is nothing in common. directives.adapt_path ends with
	// `utils.relative_path(None, base/path)` and comments why -- "convert to
	// relative path for shorter system messages".
	shown := relativeToCWD(resolved)

	raw, err := os.ReadFile(resolved)
	if err != nil {
		return []doctree.Node{sectionMessage("3", "ERROR",
			"Problems with \"include\" directive path:\n"+inputErrorString(err, shown)+".",
			lineno, blockText)}, true
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")

	text, cerr := clipInclude(text, options)
	if cerr != "" {
		return []doctree.Node{sectionMessage("3", "ERROR", cerr, lineno, blockText)}, true
	}

	tabWidth := 8
	if v, ok := options["tab-width"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			tabWidth = n
		}
	}
	if tabWidth >= 0 {
		text = expandTabs(text, tabWidth)
	}

	if _, literal := options["literal"]; literal {
		return []doctree.Node{p.includeAsLiteral(text, options, nil)}, true
	}
	if lang, ok := options["code"]; ok {
		classes := []string{"code"}
		if l := strings.TrimSpace(lang); l != "" {
			classes = append(classes, l)
		}
		return []doctree.Node{p.includeAsLiteral(strings.TrimSuffix(text, "\n"), options, classes)}, true
	}

	// Circular inclusion. The log is seeded with the document's own path, so a
	// file that includes the document including it is caught too.
	entry := includeEntry{path: shown, clip: clipKey(options)}
	if len(p.includeLog) == 0 {
		p.includeLog = append(p.includeLog, includeEntry{path: relativeToCWD(p.opts.SourcePath)})
	}
	for _, e := range p.includeLog {
		if e == entry {
			chain := []string{shown}
			for k := len(p.includeLog) - 1; k >= 0; k-- {
				chain = append(chain, p.includeLog[k].path)
			}
			return []doctree.Node{sectionMessage("2", "WARNING",
				"circular inclusion in \"include\" directive:\n"+strings.Join(chain, "\n> "),
				lineno, blockText)}, true
		}
	}
	p.includeLog = append(p.includeLog, entry)
	p.includeDirs = append(p.includeDirs, filepath.Dir(resolved))

	spliced := splitLines(text)
	// A blank line on EACH side of the marker. The leading one separates it from
	// the included content; the trailing one matters because the directive's own
	// block has already consumed the blank line that followed it, so without this
	// the marker would be followed immediately by the next line of the including
	// document -- "Explicit markup ends without a blank line; unexpected
	// unindent.", and the marker itself left in the tree as a comment, since the
	// suppression requires a blank finish exactly as Body.comment does.
	spliced = append(spliced, "", ".. "+endOfInclusionPrefix+shown+`"`, "")
	p.pendingInclude = spliced
	return nil, true
}

// includeAsLiteral builds the <literal_block> for ":literal:" or ":code:",
// including ":class:", ":name:" and ":number-lines:" the same way the code
// directive does.
func (p *parser) includeAsLiteral(text string, options map[string]string, classes []string) *doctree.Element {
	if v, ok := options["class"]; ok {
		classes = append(classes, classOption(v)...)
	}
	el := doctree.NewElement(doctree.TagLiteralBlock)
	if len(classes) > 0 {
		el.SetAttr("class", strings.Join(classes, " "))
	}
	if v, ok := options["name"]; ok && v != "" {
		name := normalizeName(v)
		el.SetAttr("name", name)
		el.SetAttr("id", p.explicitTargetID(el.Tag, name))
	}
	if v, ok := options["number-lines"]; ok {
		startline := 1
		if s := strings.TrimSpace(v); s != "" {
			if n, err := strconv.Atoi(s); err == nil {
				startline = n
			}
		}
		appendNumberedCode(el, strings.TrimSuffix(text, "\n"), startline)
		return el
	}
	el.Append(&doctree.Text{Data: text})
	return el
}

// clipInclude applies ":start-line:", ":end-line:", ":start-after:" and
// ":end-before:", in that order, and returns the message for the two that can
// fail. Read from Include.read_file: the line options slice, and the text options
// search for and REMOVE everything up to and including their match.
func clipInclude(text string, options map[string]string) (string, string) {
	startLine, hasStart := intOption(options, "start-line")
	endLine, hasEnd := intOption(options, "end-line")
	if hasStart || hasEnd {
		lines := strings.Split(text, "\n")
		from, to := 0, len(lines)
		if hasStart {
			from = clampIndex(startLine, len(lines))
		}
		if hasEnd {
			to = clampIndex(endLine, len(lines))
		}
		if from > to {
			from = to
		}
		text = strings.Join(lines[from:to], "\n")
	}
	if after, ok := options["start-after"]; ok {
		// An EMPTY ":start-after:" means "skip everything before the first
		// blank line", which the reference spells by searching for "\n\n".
		if after == "" {
			after = "\n\n"
		}
		idx := strings.Index(text, after)
		if idx < 0 {
			return "", "Problem with \"start-after\" option of \"include\" directive:\nText not found."
		}
		text = text[idx+len(after):]
	}
	if before, ok := options["end-before"]; ok {
		if before == "" {
			if idx := strings.Index(text, "\n\n"); idx > 0 {
				text = text[:idx+1]
			}
		} else {
			idx := strings.Index(text, before)
			if idx < 0 {
				return "", "Problem with \"end-before\" option of \"include\" directive:\nText not found."
			}
			text = text[:idx]
		}
	}
	return text, ""
}

// clipKey is the clipping options as one string, for the circular-inclusion key.
func clipKey(options map[string]string) string {
	return options["start-line"] + "\x00" + options["end-line"] + "\x00" +
		options["start-after"] + "\x00" + options["end-before"]
}

func intOption(options map[string]string, name string) (int, bool) {
	v, ok := options[name]
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0, false
	}
	return n, true
}

// clampIndex turns a possibly negative or oversized line index into a slice
// bound, the way Python's own slicing does.
func clampIndex(n, length int) int {
	if n < 0 {
		n += length
		if n < 0 {
			n = 0
		}
	}
	if n > length {
		n = length
	}
	return n
}

// inputErrorString reproduces the reference's io.error_string for a read failure:
// the exception class name, a colon, and Python's own errno message with the path
// quoted. docutils wraps every OSError from its FileInput in an InputError, which
// is why the class name is that and not the OS error's own.
func inputErrorString(err error, path string) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "InputError: [Errno 2] No such file or directory: '" + path + "'"
	case errors.Is(err, fs.ErrPermission):
		return "InputError: [Errno 13] Permission denied: '" + path + "'"
	default:
		return "InputError: " + err.Error()
	}
}

// spliceInclude inserts the lines an ".. include::" just read at position at, and
// returns the new slice. It is a no-op for every other directive.
//
// Splicing into the INPUT, rather than parsing the file separately and grafting its
// nodes, is what makes an included file behave as if it had been typed in place: a
// section title in it nests under the including document's sections, a construct is
// not cut at the file boundary, and ids are assigned in document order.
//
// Line numbers in a message raised inside included text are those of the splice
// position rather than of the file it came from. The reference has the same
// limitation and says so ("TODO: if startline != 0, line numbers are wrong").
func (p *parser) spliceInclude(lines []string, at int) []string {
	ins := p.pendingInclude
	if ins == nil {
		return lines
	}
	p.pendingInclude = nil
	out := make([]string, 0, len(lines)+len(ins))
	out = append(out, lines[:at]...)
	out = append(out, ins...)
	out = append(out, lines[at:]...)
	return out
}

// relativeToCWD ports utils.relative_path(None, target): the path to target from
// the process's working directory, with "/" separators -- and the ABSOLUTE path
// when the two have nothing in common, which the reference decides by comparing the
// first two components (so "/a/..." and "/b/..." already count as nothing in
// common on a POSIX system).
func relativeToCWD(target string) string {
	cwd, err := os.Getwd()
	if err != nil {
		// One error path, not two: filepath.Abs's only failure IS this one, so
		// asking twice would add a branch nothing can reach on its own.
		return filepath.ToSlash(target)
	}
	abs := target
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(cwd, abs)
	}
	// The reference compares "the first 2 parts" of each path, which for a POSIX
	// absolute path are "" and the first component.
	srcParts := strings.Split(filepath.ToSlash(filepath.Join(cwd, "dummy_file")), "/")
	dstParts := strings.Split(filepath.ToSlash(abs), "/")
	if len(srcParts) < 2 || len(dstParts) < 2 || srcParts[0] != dstParts[0] || srcParts[1] != dstParts[1] {
		return filepath.ToSlash(abs)
	}
	common := 0
	for common < len(srcParts) && common < len(dstParts) && srcParts[common] == dstParts[common] {
		common++
	}
	up := len(srcParts) - common - 1
	parts := make([]string, 0, up+len(dstParts)-common)
	for k := 0; k < up; k++ {
		parts = append(parts, "..")
	}
	parts = append(parts, dstParts[common:]...)
	return strings.Join(parts, "/")
}
