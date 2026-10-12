package custom

import (
	"fmt"
)

// Limits of one expression. A rule reads like "value > 25 and value < 30";
// these leave room for that many times over while keeping the work to
// parse and evaluate one small and bounded.
const (
	MaxExpressionLength = 500
	maxNesting          = 20
)

// SyntaxError is an expression that cannot be parsed, with the character
// (from 1) where the problem is.
type SyntaxError struct {
	Position int
	Message  string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("%s at character %d", e.Message, e.Position)
}

// Expr is a parsed expression. The grammar, loosest binding first:
//
//	or      = and { ("or" | "||") and }
//	and     = not { ("and" | "&&") not }
//	not     = ("not" | "!") not | compare
//	compare = sum [ ("==" | "!=" | "<" | "<=" | ">" | ">=") sum ]
//	sum     = product { ("+" | "-") product }
//	product = unary { ("*" | "/") unary }
//	unary   = "-" unary | primary
//	primary = number | string | true | false | null | path
//	        | name "(" [ or { "," or } ] ")" | "(" or ")"
//
// There are no loops, assignments or user-defined functions, so evaluating
// an expression visits each part of it once.
type Expr struct {
	text string
	root node
	// names are the leading keys of the paths in the expression, so a rule
	// can tell whether it reads the row of a list.
	names map[string]bool
}

// String returns the expression as it was written.
func (e *Expr) String() string { return e.text }

// Reads reports whether the expression has a path that starts with name,
// such as row.price for "row".
func (e *Expr) Reads(name string) bool { return e.names[name] }

// ParseExpr parses an expression, or returns a *SyntaxError.
func ParseExpr(src string) (*Expr, error) {
	if len(src) > MaxExpressionLength {
		return nil, &SyntaxError{Position: MaxExpressionLength + 1, Message: fmt.Sprintf("an expression may be at most %d characters", MaxExpressionLength)}
	}
	tokens, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{tokens: tokens, names: map[string]bool{}}
	root, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.kind != tokenEnd {
		return nil, p.fail(t, "expected an operator such as and, or, == or +")
	}
	return &Expr{text: src, root: root, names: p.names}, nil
}

type parser struct {
	tokens []token
	pos    int
	depth  int
	names  map[string]bool
}

func (p *parser) peek() token { return p.tokens[p.pos] }

func (p *parser) next() token {
	t := p.tokens[p.pos]
	if t.kind != tokenEnd {
		p.pos++
	}
	return t
}

func (p *parser) fail(t token, msg string) error {
	if t.kind == tokenEnd {
		msg += ", but the expression ends"
	}
	return &SyntaxError{Position: t.pos + 1, Message: msg}
}

// isOp reports whether t is the operator op.
func isOp(t token, op string) bool { return t.kind == tokenOperator && t.text == op }

// isWord reports whether t is the bare word w, such as "and".
func isWord(t token, w string) bool {
	if t.kind != tokenPath {
		return false
	}
	key, ok := t.path.first()
	return ok && key == w && len(t.path.segments) == 1
}

// enter counts one level of nesting; parentheses, calls and repeated not
// or minus each nest, and the parser stops before the stack grows large.
func (p *parser) enter(t token) error {
	p.depth++
	if p.depth > maxNesting {
		return &SyntaxError{Position: t.pos + 1, Message: fmt.Sprintf("the expression nests deeper than %d levels", maxNesting)}
	}
	return nil
}

func (p *parser) parseOr() (node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for t := p.peek(); isWord(t, "or") || isOp(t, "||"); t = p.peek() {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &logicNode{or: true, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (node, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for t := p.peek(); isWord(t, "and") || isOp(t, "&&"); t = p.peek() {
		p.next()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &logicNode{left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseNot() (node, error) {
	t := p.peek()
	if !isWord(t, "not") && !isOp(t, "!") {
		return p.parseCompare()
	}
	p.next()
	if err := p.enter(t); err != nil {
		return nil, err
	}
	operand, err := p.parseNot()
	p.depth--
	if err != nil {
		return nil, err
	}
	return &notNode{operand: operand}, nil
}

var comparisons = map[string]bool{"==": true, "!=": true, "<": true, "<=": true, ">": true, ">=": true}

func (p *parser) parseCompare() (node, error) {
	left, err := p.parseSum()
	if err != nil {
		return nil, err
	}
	t := p.peek()
	if t.kind != tokenOperator || !comparisons[t.text] {
		return left, nil
	}
	p.next()
	right, err := p.parseSum()
	if err != nil {
		return nil, err
	}
	if next := p.peek(); next.kind == tokenOperator && comparisons[next.text] {
		return nil, p.fail(next, "use and to combine comparisons, as in a < b and b < c")
	}
	return &compareNode{op: t.text, left: left, right: right}, nil
}

func (p *parser) parseSum() (node, error) {
	left, err := p.parseProduct()
	if err != nil {
		return nil, err
	}
	for t := p.peek(); isOp(t, "+") || isOp(t, "-"); t = p.peek() {
		p.next()
		right, err := p.parseProduct()
		if err != nil {
			return nil, err
		}
		left = &arithNode{op: t.text[0], left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseProduct() (node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for t := p.peek(); isOp(t, "*") || isOp(t, "/"); t = p.peek() {
		p.next()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &arithNode{op: t.text[0], left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (node, error) {
	t := p.peek()
	if !isOp(t, "-") {
		return p.parsePrimary()
	}
	p.next()
	if err := p.enter(t); err != nil {
		return nil, err
	}
	operand, err := p.parseUnary()
	p.depth--
	if err != nil {
		return nil, err
	}
	return &negateNode{operand: operand}, nil
}
