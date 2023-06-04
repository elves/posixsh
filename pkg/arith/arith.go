package arith

import (
	"fmt"
	"strconv"
	"strings"
)

func Eval(s string, variables map[string]string) (int64, error) {
	tokens, starts := lex(s)
	p := parser{0, tokens, starts, s, variables, true}
	result, err := p.top()
	if err == nil && !p.eof() {
		// TODO: Add position
		err = p.errorf("trailing content: %v", p.rest())
	}
	return result, err
}

type parser struct {
	next       int
	tokens     []string
	starts     []int
	original   string
	variables  map[string]string
	sideEffect bool
}

func (p *parser) eof() bool { return p.next == len(p.tokens) }

func (p *parser) rest() string {
	if p.next == len(p.tokens) {
		return ""
	}
	return p.original[p.starts[p.next]:]
}

func (p *parser) peek(ahead int) string {
	if p.next+ahead < len(p.tokens) {
		return p.tokens[p.next+ahead]
	}
	return ""
}

func (p *parser) nextIf(f func(s string) bool) string {
	if token := p.peek(0); token != "" && f(token) {
		p.next++
		return token
	}
	return ""
}

func (p *parser) consume(s string) bool {
	return p.nextIf(func(t string) bool { return s == t }) != ""
}

func (p *parser) errorf(format string, args ...any) error {
	// TODO: Add position
	return fmt.Errorf(format, args...)
}

func (p *parser) top() (int64, error) { return p.comma() }

func (p *parser) comma() (int64, error) {
	x, err := p.assign()
	if err != nil {
		return 0, err
	}
	for p.consume(",") {
		x, err = p.assign()
		if err != nil {
			return 0, err
		}
	}
	return x, nil
}

func (p *parser) assign() (int64, error) {
	if name, op, ok := p.lookaheadAssign(); ok {
		if !isVariableToken(name) {
			p.errorf("cannot assign to non-variable: %q", name)
		}
		if op != "" && binaryOps[op] == nil {
			p.errorf("unknown operator in augmented assignment: %q", op)
		}
		rhs, err := p.assign()
		if err != nil {
			return 0, err
		}
		if !p.sideEffect {
			// Evaluating the non-active branch of a conditional expression. The
			// return value doesn't matter.
			return 0, nil
		}
		var result int64
		if op == "" {
			result = rhs
		} else {
			current, err := p.getVar(name)
			if err != nil {
				return 0, err
			}
			result = binaryOps[op](current, rhs)
		}
		p.variables[name] = strconv.FormatInt(result, 10)
		return result, nil
	}
	return p.condition()
}

func (p *parser) lookaheadAssign() (name, op string, ok bool) {
	i := p.next
	if p.peek(1) == "=" {
		p.next += 2
		return p.tokens[i], "", true
	} else if p.peek(2) == "=" {
		p.next += 3
		return p.tokens[i], p.tokens[i+1], true
	}
	return "", "", false
}

func (p *parser) condition() (int64, error) {
	x, err := p.logicalOr()
	if err != nil {
		return 0, err
	}
	if !p.consume("?") {
		return x, nil
	}
	cond := i2b(x)

	p.sideEffect = cond
	trueVal, err := p.top()
	p.sideEffect = true
	if err != nil {
		return 0, err
	}
	if !p.consume(":") {
		return 0, p.errorf("expect ':' as part of conditional expression")
	}

	p.sideEffect = !cond
	falseVal, err := p.top()
	p.sideEffect = true
	if err != nil {
		return 0, err
	}

	if cond {
		return trueVal, nil
	} else {
		return falseVal, nil
	}
}

var binaryOps = map[string]func(x, y int64) int64{
	"||": func(x, y int64) int64 { return b2i(i2b(x) || i2b(y)) },
	"&&": func(x, y int64) int64 { return b2i(i2b(x) && i2b(y)) },
	"|":  func(x, y int64) int64 { return x | y },
	"^":  func(x, y int64) int64 { return x ^ y },
	"&":  func(x, y int64) int64 { return x & y },
	"==": func(x, y int64) int64 { return b2i(x == y) },
	"!=": func(x, y int64) int64 { return b2i(x != y) },
	"<=": func(x, y int64) int64 { return b2i(x <= y) },
	"<":  func(x, y int64) int64 { return b2i(x < y) },
	">=": func(x, y int64) int64 { return b2i(x >= y) },
	">":  func(x, y int64) int64 { return b2i(x > y) },
	"<<": func(x, y int64) int64 { return x << y },
	">>": func(x, y int64) int64 { return x >> y },
	"+":  func(x, y int64) int64 { return x + y },
	"-":  func(x, y int64) int64 { return x - y },
	"*":  func(x, y int64) int64 { return x * y },
	"/":  func(x, y int64) int64 { return x / y },
	"%":  func(x, y int64) int64 { return x % y },
}

func (p *parser) binaryLtr(next func() (int64, error), ops ...string) (int64, error) {
	x, err := next()
	if err != nil {
		return 0, err
	}
operand:
	for {
		for _, op := range ops {
			if p.consume(op) {
				y, err := next()
				if err != nil {
					return 0, err
				}
				x = binaryOps[op](x, y)
				continue operand
			}
		}
		break
	}
	return x, nil
}

func (p *parser) logicalOr() (int64, error) {
	return p.binaryLtr(p.logicalAnd, "||")
}

func (p *parser) logicalAnd() (int64, error) {
	return p.binaryLtr(p.bitwiseOr, "&&")
}

func (p *parser) bitwiseOr() (int64, error) {
	return p.binaryLtr(p.bitwiseXor, "|")
}

func (p *parser) bitwiseXor() (int64, error) {
	return p.binaryLtr(p.bitwiseAnd, "^")
}

func (p *parser) bitwiseAnd() (int64, error) {
	return p.binaryLtr(p.eqNe, "&")
}

func (p *parser) eqNe() (int64, error) {
	return p.binaryLtr(p.leLtGeGt, "==", "!=")
}

func (p *parser) leLtGeGt() (int64, error) {
	return p.binaryLtr(p.bitwiseShift, "<=", "<", ">=", ">")
}

func (p *parser) bitwiseShift() (int64, error) {
	return p.binaryLtr(p.addSub, "<<", ">>")
}

func (p *parser) addSub() (int64, error) {
	return p.binaryLtr(p.mulDivMod, "+", "-")
}

func (p *parser) mulDivMod() (int64, error) {
	return p.binaryLtr(p.primary, "*", "/", "%")
}

func (p *parser) primary() (int64, error) {
	if p.consume("(") {
		// '(' expr ')'
		x, err := p.top()
		if err == nil {
			if !p.consume(")") {
				err = p.errorf("unclosed (")
			}
		}
		return x, err
	} else if p.consume("~") {
		x, err := p.primary()
		return ^x, err
	} else if p.consume("!") {
		x, err := p.primary()
		return not(x), err
	} else if p.consume("+") {
		return p.primary()
	} else if p.consume("-") {
		x, err := p.primary()
		return -x, err
	} else if num := p.nextIf(isNumberToken); num != "" {
		x, ok := parseNum(num)
		if !ok {
			return 0, p.errorf("can't parse as number %q", num)
		}
		return x, nil
	} else if token := p.peek(0); token == "--" || token == "++" || (token != "" && isVariableToken(token)) {
		// Evaluate a variable, possibly surrounded by ++ and -- on either side.
		preInc, preDec := p.parseIncDec()
		name := p.nextIf(isVariableToken)
		if name == "" {
			return 0, p.errorf("expect variable name from %q", p.rest())
		}
		postInc, postDec := p.parseIncDec()
		if !p.sideEffect {
			return 0, nil
		}
		oldVal, err := p.getVar(name)
		if err != nil {
			return 0, err
		}
		newVal := oldVal + int64(preInc+postInc-preDec-postDec)
		p.variables[name] = strconv.FormatInt(newVal, 10)
		// The value of an expression with both prefix increment/decrement
		// operators and postfix increment/decrement operators is not
		// well-defined. We use a simple rule: if there is any prefix operator,
		// the value is the new value of the variable; otherwise it's the old
		// value of the variable.
		if preInc > 0 || preDec > 0 {
			return newVal, nil
		}
		return oldVal, nil
	} else {
		return 0, p.errorf("can't parse a primary expression from %q", p.rest())
	}
}

func (p *parser) parseIncDec() (inc, dec int) {
	for !p.eof() {
		if p.consume("++") {
			inc++
		} else if p.consume("--") {
			dec++
		} else {
			break
		}
	}
	return
}

func (p *parser) getVar(name string) (int64, error) {
	value := p.variables[name]
	if value == "" {
		// Not defined in POSIX, but all of dash, bash and zsh treat unset
		// and empty variables as 0.
		return 0, nil
	} else if n, ok := parseNum(value); ok {
		// This is the only case defined by POSIX - a variable containing a
		// valid literal.
		return n, nil
	}
	// When the value is non-empty but can't be parsed as a number, dash
	// errors, while bash and zsh treat the content as another arithmetic
	// expression and evaluate it recursively (subject to a recursion depth
	// limit). We follow dash for simplicity.
	return 0, p.errorf("$%s not a number: %q", name, value)
}

func parseNum(s string) (int64, bool) {
	var neg bool
	if strings.HasPrefix(s, "+") {
		s = s[1:]
	} else if strings.HasPrefix(s, "-") {
		s = s[1:]
		neg = true
	}

	var n int64
	var err error
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		n, err = strconv.ParseInt(s[2:], 16, 64)
	} else if strings.HasPrefix(s, "0") {
		if s == "0" {
			// +0 and -0 are also just 0
			return 0, true
		} else {
			n, err = strconv.ParseInt(s[1:], 8, 64)
		}
	} else {
		n, err = strconv.ParseInt(s, 10, 64)
	}
	if neg {
		n = -n
	}
	return n, err == nil
}

func not(i int64) int64 {
	if i == 0 {
		return 1
	}
	return 0
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func i2b(i int64) bool {
	return i != 0
}
