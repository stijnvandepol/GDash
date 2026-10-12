package custom

import "strings"

type tokenKind int

const (
	tokenEnd tokenKind = iota
	tokenNumber
	tokenString
	// tokenPath is a path, which includes bare words such as and, true or
	// the name of a function; the parser tells them apart.
	tokenPath
	tokenOperator
)

type token struct {
	kind tokenKind
	// text is the operator, the digits of a number or a string's content.
	text string
	path Path
	// pos is the byte offset in the expression, for error messages.
	pos int
}

// twoCharOperators are tried before the one-character ones, so <= is not
// read as < followed by =.
var twoCharOperators = []string{"==", "!=", "<=", ">=", "&&", "||"}

const oneCharOperators = "()+-*/<>!,"

// lex splits an expression into tokens, ending with a tokenEnd.
func lex(src string) ([]token, error) {
	var tokens []token
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c >= '0' && c <= '9':
			end := numberEnd(src, i)
			tokens = append(tokens, token{kind: tokenNumber, text: src[i:end], pos: i})
			i = end
		case c == '"' || c == '\'':
			text, end, err := scanQuoted(src, i)
			if err != nil {
				return nil, &SyntaxError{Position: i + 1, Message: "this text has no closing quote or an unknown escape"}
			}
			tokens = append(tokens, token{kind: tokenString, text: text, pos: i})
			i = end
		case isNameStart(c) || c == '$' || c == '[':
			p, n, err := scanPath(src[i:])
			if err != nil {
				return nil, &SyntaxError{Position: i + 1, Message: err.Error()}
			}
			tokens = append(tokens, token{kind: tokenPath, text: p.text, path: p, pos: i})
			i += n
		default:
			op := operatorAt(src, i)
			if op == "" {
				return nil, &SyntaxError{Position: i + 1, Message: "unexpected character " + quoteByte(c)}
			}
			tokens = append(tokens, token{kind: tokenOperator, text: op, pos: i})
			i += len(op)
		}
	}
	return append(tokens, token{kind: tokenEnd, pos: len(src)}), nil
}

func operatorAt(src string, i int) string {
	for _, op := range twoCharOperators {
		if strings.HasPrefix(src[i:], op) {
			return op
		}
	}
	if strings.IndexByte(oneCharOperators, src[i]) >= 0 {
		return src[i : i+1]
	}
	return ""
}

// numberEnd returns the end of a number such as 12, 0.5 or 1e6.
func numberEnd(src string, i int) int {
	digits := func(j int) int {
		for j < len(src) && src[j] >= '0' && src[j] <= '9' {
			j++
		}
		return j
	}
	i = digits(i)
	if i+1 < len(src) && src[i] == '.' && src[i+1] >= '0' && src[i+1] <= '9' {
		i = digits(i + 1)
	}
	if i < len(src) && (src[i] == 'e' || src[i] == 'E') {
		j := i + 1
		if j < len(src) && (src[j] == '+' || src[j] == '-') {
			j++
		}
		if j < len(src) && src[j] >= '0' && src[j] <= '9' {
			i = digits(j)
		}
	}
	return i
}

func quoteByte(c byte) string {
	if c < 0x20 || c >= 0x7f {
		return "(not plain text)"
	}
	return "'" + string(c) + "'"
}
