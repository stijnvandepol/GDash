// Package custom shows data from any JSON HTTP API that an admin sets up in
// the web interface: the sources (address, authentication, refresh), the
// path syntax that picks values out of an answer, a small expression
// language for rules, and the views that the Custom widget shows. Nothing
// an admin types is ever run as code: paths and expressions are parsed into
// a fixed set of operations that cannot loop, assign or reach anything but
// the answer they are given.
package custom

import (
	"fmt"
	"strconv"
	"strings"
)

// MaxPathLength bounds a path in characters; real paths are a few dozen.
const MaxPathLength = 200

// maxSegments bounds the steps of one path, so resolving it is cheap.
const maxSegments = 32

// Path is a parsed path into a JSON document, such as data.items[0].name.
// The syntax:
//
//	name          a key of an object; letters, digits and _ only
//	.name         a key after another step
//	["any key"]   a key with other characters, in double or single quotes
//	[2], [-1]     an element of a list, from the start or from the end
//	[*]           every element of a list; the path then yields a list
//	$             the whole document; $.name and $[0] start from it
type Path struct {
	text     string
	rooted   bool
	segments []segment
}

type segmentKind int

const (
	segmentKey segmentKind = iota
	segmentIndex
	segmentAll
)

type segment struct {
	kind  segmentKind
	key   string
	index int
}

// String returns the path as it was written.
func (p Path) String() string { return p.text }

// first returns the leading key of a path that does not start at $, which
// may name a variable such as value or row.
func (p Path) first() (string, bool) {
	if p.rooted || len(p.segments) == 0 || p.segments[0].kind != segmentKey {
		return "", false
	}
	return p.segments[0].key, true
}

// ParsePath parses a whole string as a path.
func ParsePath(s string) (Path, error) {
	if s == "" {
		return Path{}, fmt.Errorf("the path is empty")
	}
	p, n, err := scanPath(s)
	if err != nil {
		return Path{}, err
	}
	if n != len(s) {
		return Path{}, fmt.Errorf("unexpected %q at character %d of the path", s[n], n+1)
	}
	return p, nil
}

// scanPath reads the longest path at the start of s and returns how many
// bytes it used. The expression parser uses it for paths inside rules, so
// both share one syntax.
func scanPath(s string) (Path, int, error) {
	if len(s) > MaxPathLength {
		return Path{}, 0, fmt.Errorf("a path may be at most %d characters", MaxPathLength)
	}
	p := Path{}
	i := 0
	switch {
	case s[0] == '$':
		p.rooted = true
		i = 1
	case isNameStart(s[0]):
		end := nameEnd(s, 0)
		p.segments = append(p.segments, segment{kind: segmentKey, key: s[:end]})
		i = end
	case s[0] != '[':
		return Path{}, 0, fmt.Errorf("a path starts with a name, [ or $")
	}
	for i < len(s) {
		var seg segment
		var next int
		var err error
		switch s[i] {
		case '.':
			if i+1 >= len(s) || !isNameStart(s[i+1]) {
				return Path{}, 0, fmt.Errorf("expected a name after the dot at character %d", i+1)
			}
			next = nameEnd(s, i+1)
			seg = segment{kind: segmentKey, key: s[i+1 : next]}
		case '[':
			seg, next, err = scanBracket(s, i)
			if err != nil {
				return Path{}, 0, err
			}
		default:
			return finishPath(p, s, i)
		}
		if len(p.segments) == maxSegments {
			return Path{}, 0, fmt.Errorf("a path may have at most %d steps", maxSegments)
		}
		p.segments = append(p.segments, seg)
		i = next
	}
	return finishPath(p, s, i)
}

func finishPath(p Path, s string, n int) (Path, int, error) {
	p.text = s[:n]
	return p, n, nil
}

// scanBracket reads [n], [*] or ["key"] at s[i].
func scanBracket(s string, i int) (segment, int, error) {
	j := i + 1
	if j < len(s) && s[j] == '*' {
		if j+1 < len(s) && s[j+1] == ']' {
			return segment{kind: segmentAll}, j + 2, nil
		}
		return segment{}, 0, fmt.Errorf("expected ] after [* at character %d", j+2)
	}
	if j < len(s) && (s[j] == '"' || s[j] == '\'') {
		key, end, err := scanQuoted(s, j)
		if err != nil {
			return segment{}, 0, err
		}
		if end >= len(s) || s[end] != ']' {
			return segment{}, 0, fmt.Errorf("expected ] after the quoted key at character %d", end+1)
		}
		return segment{kind: segmentKey, key: key}, end + 1, nil
	}
	end := j
	if end < len(s) && s[end] == '-' {
		end++
	}
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end >= len(s) || s[end] != ']' {
		return segment{}, 0, fmt.Errorf("expected a whole number, * or a quoted key in [ ] at character %d", i+1)
	}
	index, err := strconv.Atoi(s[j:end])
	if err != nil || index > 1_000_000 || index < -1_000_000 {
		return segment{}, 0, fmt.Errorf("the list index at character %d is not a usable whole number", i+2)
	}
	return segment{kind: segmentIndex, index: index}, end + 1, nil
}

// scanQuoted reads a string in single or double quotes at s[i], with \\,
// \" and \' as escapes. It returns the text and the index after the quote.
func scanQuoted(s string, i int) (string, int, error) {
	quote := s[i]
	var b strings.Builder
	for j := i + 1; j < len(s); j++ {
		switch c := s[j]; {
		case c == quote:
			return b.String(), j + 1, nil
		case c == '\\' && j+1 < len(s):
			j++
			switch s[j] {
			case 'n':
				b.WriteByte('\n')
			case '\\', '"', '\'':
				b.WriteByte(s[j])
			default:
				return "", 0, fmt.Errorf("unknown escape \\%c at character %d", s[j], j)
			}
		default:
			b.WriteByte(c)
		}
	}
	return "", 0, fmt.Errorf("the text starting at character %d has no closing quote", i+1)
}

func isNameStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func nameEnd(s string, i int) int {
	for i < len(s) && (isNameStart(s[i]) || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	return i
}

// Resolve returns the value at p in doc, which holds what encoding/json
// decodes into an any. found is false when a key or element is missing. A
// path with [*] always yields a list, of every element it reaches.
func (p Path) Resolve(doc any) (value any, found bool) {
	return resolveSegments(doc, p.segments)
}

// resolveFrom resolves p with variables: a path whose first key names one
// of vars, such as value or row, starts from that variable instead.
func (p Path) resolveFrom(doc any, vars map[string]any) (any, bool) {
	if key, ok := p.first(); ok {
		if v, bound := vars[key]; bound {
			return resolveSegments(v, p.segments[1:])
		}
	}
	return resolveSegments(doc, p.segments)
}

// resolveSegments walks the document one step at a time. Each step visits
// every current node once, and the nodes of one step are distinct, so the
// work is at most the size of the document per step.
func resolveSegments(doc any, segments []segment) (any, bool) {
	current := []any{doc}
	many := false
	for _, seg := range segments {
		var next []any
		for _, v := range current {
			next = step(next, v, seg)
		}
		if seg.kind == segmentAll {
			many = true
		}
		current = next
	}
	if many {
		if current == nil {
			current = []any{}
		}
		return current, true
	}
	if len(current) != 1 {
		return nil, false
	}
	return current[0], true
}

func step(out []any, v any, seg segment) []any {
	switch seg.kind {
	case segmentKey:
		if obj, ok := v.(map[string]any); ok {
			if child, ok := obj[seg.key]; ok {
				out = append(out, child)
			}
		}
	case segmentIndex:
		if list, ok := v.([]any); ok {
			i := seg.index
			if i < 0 {
				i += len(list)
			}
			if i >= 0 && i < len(list) {
				out = append(out, list[i])
			}
		}
	case segmentAll:
		if list, ok := v.([]any); ok {
			out = append(out, list...)
		}
	}
	return out
}
