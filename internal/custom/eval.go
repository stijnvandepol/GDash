package custom

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// Env is what an expression is evaluated against.
type Env struct {
	// Doc is the API's answer, as encoding/json decodes it into an any.
	Doc any
	// Vars are names that a path may start with instead of a key of Doc,
	// such as value (the widget's value) or row (one row of a list).
	Vars map[string]any
	// Now is the time minutesSince counts from.
	Now time.Time
}

// maxTextResult bounds text that + builds, so joining long values from an
// answer cannot use much memory.
const maxTextResult = 4096

// Eval evaluates the expression. Values are float64, string, bool, nil,
// []any or map[string]any. An error means the expression does not apply
// to this data, such as comparing a missing value with a number.
func (e *Expr) Eval(env Env) (any, error) {
	return e.root.eval(&env)
}

// Matches evaluates a rule's condition: true only when the expression
// yields true. Any other value is an error, so a typo such as "value" for
// "value > 3" is reported instead of quietly matching.
func (e *Expr) Matches(env Env) (bool, error) {
	v, err := e.Eval(env)
	if err != nil {
		return false, err
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("the rule gives %s, not true or false", describe(v))
	}
	return b, nil
}

type node interface {
	eval(env *Env) (any, error)
}

type literalNode struct{ value any }

func (n *literalNode) eval(*Env) (any, error) { return n.value, nil }

type pathNode struct{ path Path }

// eval yields null for a missing value, so "x == null" can test for it.
func (n *pathNode) eval(env *Env) (any, error) {
	v, _ := n.path.resolveFrom(env.Doc, env.Vars)
	return v, nil
}

type notNode struct{ operand node }

func (n *notNode) eval(env *Env) (any, error) {
	b, err := evalBool(n.operand, env, "not")
	return !b, err
}

type logicNode struct {
	or          bool
	left, right node
}

// eval stops after the left side when it decides the answer, as in most
// languages: "x != null and x > 3" is safe when x is missing.
func (n *logicNode) eval(env *Env) (any, error) {
	word := "and"
	if n.or {
		word = "or"
	}
	left, err := evalBool(n.left, env, word)
	if err != nil || left == n.or {
		return left, err
	}
	return evalBool(n.right, env, word)
}

func evalBool(n node, env *Env, word string) (bool, error) {
	v, err := n.eval(env)
	if err != nil {
		return false, err
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("%s needs true or false, not %s", word, describe(v))
	}
	return b, nil
}

type negateNode struct{ operand node }

func (n *negateNode) eval(env *Env) (any, error) {
	v, err := n.operand.eval(env)
	if err != nil {
		return nil, err
	}
	f, ok := v.(float64)
	if !ok {
		return nil, fmt.Errorf("- needs a number, not %s", describe(v))
	}
	return -f, nil
}

type arithNode struct {
	op          byte
	left, right node
}

func (n *arithNode) eval(env *Env) (any, error) {
	l, err := n.left.eval(env)
	if err != nil {
		return nil, err
	}
	r, err := n.right.eval(env)
	if err != nil {
		return nil, err
	}
	if ls, ok := l.(string); ok && n.op == '+' {
		rs, ok := r.(string)
		if !ok {
			return nil, fmt.Errorf("+ joins text with text, not with %s", describe(r))
		}
		if len(ls)+len(rs) > maxTextResult {
			return nil, errors.New("the joined text is too long")
		}
		return ls + rs, nil
	}
	lf, lok := l.(float64)
	rf, rok := r.(float64)
	if !lok || !rok {
		return nil, fmt.Errorf("%c needs two numbers, not %s and %s", n.op, describe(l), describe(r))
	}
	return finite(arith(n.op, lf, rf))
}

func arith(op byte, l, r float64) (float64, error) {
	switch op {
	case '+':
		return l + r, nil
	case '-':
		return l - r, nil
	case '*':
		return l * r, nil
	}
	if r == 0 {
		return 0, errors.New("division by zero")
	}
	return l / r, nil
}

// finite turns an overflow into an error: JSON has no infinity, and a rule
// that compares with one is a mistake.
func finite(f float64, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return nil, errors.New("the number is too large")
	}
	return f, nil
}

type compareNode struct {
	op          string
	left, right node
}

func (n *compareNode) eval(env *Env) (any, error) {
	l, err := n.left.eval(env)
	if err != nil {
		return nil, err
	}
	r, err := n.right.eval(env)
	if err != nil {
		return nil, err
	}
	switch n.op {
	case "==":
		return equal(l, r), nil
	case "!=":
		return !equal(l, r), nil
	}
	c, err := order(l, r, n.op)
	if err != nil {
		return nil, err
	}
	switch n.op {
	case "<":
		return c < 0, nil
	case "<=":
		return c <= 0, nil
	case ">":
		return c > 0, nil
	}
	return c >= 0, nil
}

// equal compares two plain values. Lists and objects equal nothing, not
// even themselves: comparing them whole is never what a rule means.
func equal(l, r any) bool {
	switch l.(type) {
	case []any, map[string]any:
		return false
	}
	switch r.(type) {
	case []any, map[string]any:
		return false
	}
	return l == r
}

// order compares two numbers or two texts, and fails for anything else,
// such as a number with a missing value.
func order(l, r any, op string) (int, error) {
	switch lv := l.(type) {
	case float64:
		if rv, ok := r.(float64); ok {
			switch {
			case lv < rv:
				return -1, nil
			case lv > rv:
				return 1, nil
			}
			return 0, nil
		}
	case string:
		if rv, ok := r.(string); ok {
			switch {
			case lv < rv:
				return -1, nil
			case lv > rv:
				return 1, nil
			}
			return 0, nil
		}
	}
	return 0, fmt.Errorf("%s compares two numbers or two texts, not %s and %s", op, describe(l), describe(r))
}

type callNode struct {
	name string
	fn   *function
	args []node
}

func (n *callNode) eval(env *Env) (any, error) {
	values := make([]any, len(n.args))
	for i, arg := range n.args {
		v, err := arg.eval(env)
		if err != nil {
			return nil, err
		}
		values[i] = v
	}
	v, err := n.fn.call(values, env.Now)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", n.name, err)
	}
	return v, nil
}

// describe names a value's kind for an error message, without its content:
// an error may be shown in the editor, and a value could be long.
func describe(v any) string {
	switch v.(type) {
	case nil:
		return "a missing value (null)"
	case bool:
		return "true or false"
	case float64:
		return "a number"
	case string:
		return "text"
	case []any:
		return "a list"
	case map[string]any:
		return "an object"
	}
	return "an unknown value"
}
