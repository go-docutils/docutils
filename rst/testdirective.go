package rst

import (
	"sort"
	"strings"

	"github.com/go-docutils/docutils/doctree"
)

// testDirectiveName is the one directive in docutils' registry that
// exists to report on the PARSE rather than to build anything:
// misc.TestDirective, registered as a standard entry alongside the other
// 42 (not, as this project assumed for a long time, only inside
// docutils' own test suite -- the same wrong assumption v0.75.0 found
// for restructuredtext-unimplemented-role).
//
// It echoes back exactly how the directive block was split, which makes
// its twelve corpus fixtures a conformance suite for parseDirectiveBlock
// itself: no argument, an argument with an optional space before "::",
// an option on the directive's own line, an option value spanning
// several lines, content after one blank line and after two, and the
// shape where the "content" is really the ARGUMENT because no blank line
// separates it.
const testDirectiveName = "restructuredtext-test-directive"

// runTestDirective ports misc.TestDirective.run. Its declaration is
// optional_arguments=1 with final_argument_whitespace, one option
// ("option", unchanged_required) and has_content.
func runTestDirective(name, args string, body []string, blanks int, blockText string, lineno int) []doctree.Node {
	// gatherExplicitBody has already dropped the blank line(s) between a
	// same-line argument and the indented block below it, and without
	// them parseDirectiveBlock cannot tell an ARGUMENT from CONTENT: the
	// three fixtures with a genuinely separated content block all came
	// back reporting it as the argument instead. Reinserted here, the
	// same way parseSubstitutionDef and runAdmonitionOrGeneric already
	// do -- this is the third caller to need it.
	combined := make([]string, 0, 1+blanks+len(body))
	combined = append(combined, args)
	for k := 0; k < blanks; k++ {
		combined = append(combined, "")
	}
	combined = append(combined, body...)
	argument, options, content := parseDirectiveBlock(combined, true)

	// option_spec has exactly one entry, so validating it is the same
	// small port target-notes already needed (v0.74.0).
	keys := make([]string, 0, len(options))
	for k := range options {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch {
		case !strings.EqualFold(k, "option"):
			return []doctree.Node{sectionMessage("3", "ERROR",
				`Error in "`+name+`" directive:`+"\n"+`unknown option: "`+k+`".`, lineno, blockText)}
		case strings.TrimSpace(options[k]) == "":
			return []doctree.Node{sectionMessage("3", "ERROR",
				`Error in "`+name+`" directive:`+"\n"+
					`invalid option value: (option: "`+strings.ToLower(k)+`"; value: None)`+"\n"+
					`argument required but none supplied.`, lineno, blockText)}
		}
	}

	var arguments []string
	if argument != "" {
		arguments = append(arguments, argument)
	}
	text := `Directive processed. Type="` + name + `", arguments=` + pyReprList(arguments) +
		`, options=` + pyReprDict(options, keys) + `, content:`
	if len(content) == 0 {
		return []doctree.Node{sectionMessage("1", "INFO", text+" None", lineno, "")}
	}
	// With content the message keeps its bare "content:" and the content
	// itself follows as a <literal_block>, VERBATIM -- backslashes and
	// inline-markup characters included, since nothing parses it.
	return []doctree.Node{sectionMessage("1", "INFO", text, lineno, strings.Join(content, "\n"))}
}

// pyReprDict renders an option map as Python's %r would print the dict
// misc.TestDirective is handed. keys carries the iteration order (a
// Python dict prints in insertion order; sorted is the one stable order
// available here, and no fixture holds more than a single option).
func pyReprDict(options map[string]string, keys []string) string {
	if len(keys) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, pyRepr(k)+": "+pyRepr(options[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
