package custom

import (
	"fmt"
	"strconv"
)

// The innermost parts of an expression: literals, paths, calls and groups.

func (p *parser) parsePrimary() (node, error) {
	t := p.next()
	switch t.kind {
	case tokenNumber:
		n, err := strconv.ParseFloat(t.text, 64)
		if err != nil {
			return nil, p.fail(t, "this number is too large")
		}
		return &literalNode{value: n}, nil
	case tokenString:
		return &literalNode{value: t.text}, nil
	case tokenPath:
		return p.parsePathOrWord(t)
	case tokenOperator:
		if isOp(t, "(") {
			return p.parseGroup(t)
		}
	}
	return nil, p.fail(t, "expected a number, text in quotes, a path or (")
}

func (p *parser) parsePathOrWord(t token) (node, error) {
	switch {
	case isWord(t, "true"):
		return &literalNode{value: true}, nil
	case isWord(t, "false"):
		return &literalNode{value: false}, nil
	case isWord(t, "null"):
		return &literalNode{value: nil}, nil
	case isWord(t, "and") || isWord(t, "or") || isWord(t, "not"):
		return nil, p.fail(t, "expected a value before "+t.text)
	}
	if isOp(p.peek(), "(") && len(t.path.segments) == 1 && !t.path.rooted {
		return p.parseCall(t)
	}
	if key, ok := t.path.first(); ok {
		p.names[key] = true
	}
	return &pathNode{path: t.path}, nil
}

func (p *parser) parseGroup(open token) (node, error) {
	if err := p.enter(open); err != nil {
		return nil, err
	}
	inner, err := p.parseOr()
	p.depth--
	if err != nil {
		return nil, err
	}
	if t := p.next(); !isOp(t, ")") {
		return nil, p.fail(t, "expected )")
	}
	return inner, nil
}

func (p *parser) parseCall(name token) (node, error) {
	key, _ := name.path.first()
	fn, ok := functions[key]
	if !ok {
		return nil, p.fail(name, fmt.Sprintf("unknown function %s; use one of %s", key, functionNames()))
	}
	open := p.next()
	if err := p.enter(open); err != nil {
		return nil, err
	}
	defer func() { p.depth-- }()
	var args []node
	if isOp(p.peek(), ")") {
		p.next()
	} else {
		for {
			arg, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			t := p.next()
			if isOp(t, ")") {
				break
			}
			if !isOp(t, ",") {
				return nil, p.fail(t, "expected , or ) in the call of "+key)
			}
		}
	}
	if len(args) < fn.minArgs || len(args) > fn.maxArgs {
		return nil, p.fail(name, fmt.Sprintf("%s takes %s", key, fn.arity()))
	}
	return &callNode{name: key, fn: fn, args: args}, nil
}
