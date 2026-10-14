// Package linguist tells which files of a repository GitHub's Linguist treats as generated or vendored, from
// the linguist-generated and linguist-vendored attributes its .gitattributes files give them.
package linguist

import (
	"cmp"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitattributes"
)

// FileName is the name of the files declaring git attributes, at most one per directory.
const FileName = ".gitattributes"

// excludingAttributes exclude a path when set or given any value but false.
var excludingAttributes = []string{"linguist-generated", "linguist-vendored"}

// Matcher tells whether a path is marked linguist-generated or linguist-vendored.
type Matcher struct {
	matchers map[string]gitattributes.Matcher // By excluding attribute, for those some line sets
}

// NewMatcher builds a Matcher from the content of .gitattributes files, keyed by their path in the
// repository (e.g. ".gitattributes", "go/.gitattributes"). As in git, deeper files take precedence over
// shallower ones, later lines over earlier ones, and macros are only allowed in the root file. It returns
// nil when no line excludes any path.
func NewMatcher(files map[string]string) (*Matcher, error) {
	// go-git's matcher gives precedence to the end of its stack: shallower files first, each in line order
	paths := slices.SortedFunc(maps.Keys(files), func(a, b string) int {
		return cmp.Or(cmp.Compare(strings.Count(a, "/"), strings.Count(b, "/")), strings.Compare(a, b))
	})

	var stack []gitattributes.MatchAttribute
	for _, p := range paths {
		var domain []string
		if dir := path.Dir(p); dir != "." {
			domain = strings.Split(dir, "/")
		}

		lines, err := readAttributes(files[p], domain)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", p, err)
		}
		stack = append(stack, lines...)
	}

	m := &Matcher{matchers: make(map[string]gitattributes.Matcher)}
	for _, name := range excludingAttributes {
		lines := linesDeciding(stack, name)
		if slices.ContainsFunc(lines, func(line gitattributes.MatchAttribute) bool { return excludes(line.Attributes[0]) }) {
			m.matchers[name] = gitattributes.NewMatcher(lines)
		}
	}
	if len(m.matchers) == 0 {
		return nil, nil
	}

	return m, nil
}

// Excludes reports whether file, a path relative to the repository root, is marked linguist-generated or
// linguist-vendored.
func (m *Matcher) Excludes(file string) bool {
	parts := strings.Split(file, "/")
	for name, matcher := range m.matchers {
		if results, _ := matcher.Match(parts, []string{name}); results[name] != nil && excludes(results[name]) {
			return true
		}
	}

	return false
}

// readAttributes parses a .gitattributes file of the directory domain. go-git's parser panics on a line made
// of an empty quoted pattern (""): the file comes from the repository, so that is turned into an error.
func readAttributes(content string, domain []string) (lines []gitattributes.MatchAttribute, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("parser panicked: %v", r)
		}
	}()

	return gitattributes.ReadAttributes(strings.NewReader(content), domain, len(domain) == 0)
}

// linesDeciding keeps the lines of stack giving attribute name a state, reduced to that state: the last one
// the line gives it, macros the line sets expanded. go-git's Matcher walks its stack from the end but lets a
// lower priority line overwrite what a higher priority one gave unless it has found every attribute asked
// for by then. Asked for name against lines holding only name, the highest priority match decides, as in git.
func linesDeciding(stack []gitattributes.MatchAttribute, name string) []gitattributes.MatchAttribute {
	macros := make(map[string]gitattributes.Attribute)
	for _, line := range stack {
		if state := stateOf(line.Attributes, name, nil); line.Pattern == nil && state != nil {
			macros[line.Name] = state
		}
	}

	var lines []gitattributes.MatchAttribute
	for _, line := range stack {
		if state := stateOf(line.Attributes, name, macros); line.Pattern != nil && state != nil {
			lines = append(lines, gitattributes.MatchAttribute{Name: line.Name, Pattern: line.Pattern, Attributes: []gitattributes.Attribute{state}})
		}
	}

	return lines
}

// stateOf returns the state attributes give name, the last mention winning, or nil when none mentions it.
func stateOf(attributes []gitattributes.Attribute, name string, macros map[string]gitattributes.Attribute) gitattributes.Attribute {
	var state gitattributes.Attribute
	for _, attr := range attributes {
		if attr.Name() == name {
			state = attr
		} else if macro := macros[attr.Name()]; macro != nil && attr.IsSet() {
			state = macro
		}
	}

	return state
}

// excludes reports whether attr excludes a path: set or given any value but false, as Linguist's
// boolean_attribute reads it. Unset, unspecified or false doesn't.
func excludes(attr gitattributes.Attribute) bool {
	return attr.IsSet() || (attr.IsValueSet() && attr.Value() != "false")
}
