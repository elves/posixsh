package arith

import (
	"fmt"
	"strconv"
)

func Eval(s string) (int64, error) {
	p := parser{basicParser{s, 0}}
	result, err := p.expr()
	if !p.eof() {
		// TODO: Add position
		return result, p.errorf("trailing content: %v", p.rest())
	}
	return result, err
}

type parser struct {
	basicParser
}

func (p *parser) errorf(format string, args ...any) error {
	// TODO: Add position
	return fmt.Errorf(format, args...)
}

func (p *parser) expr() (int64, error) {
	acc := int64(0)
	op := "+"
	for !p.eof() && op != "" {
		t, err := p.term()
		if err != nil {
			return acc, err
		}
		switch op {
		case "+":
			acc += t
		case "-":
			acc -= t
		}
		op = p.consumePrefixIn("+", "-")
	}
	return acc, nil
}

func (p *parser) term() (int64, error) {
	acc := int64(1)
	op := "*"
	for !p.eof() && op != "" {
		f, err := p.factor()
		if err != nil {
			return acc, err
		}
		switch op {
		case "*":
			acc *= f
		case "/":
			acc /= f
		case "%":
			acc %= f
		}
		op = p.consumePrefixIn("*", "/", "%")
	}
	return acc, nil
}

const (
	octalDigitsSet       = "01234567"
	decimalDigitsSet     = octalDigitsSet + "89"
	hexadecimalDigitsSet = decimalDigitsSet + "abcdefABCDEF"
)

func (p *parser) factor() (int64, error) {
	if p.consumePrefix("(") {
		// '(' expr ')'
		v, err := p.expr()
		if err == nil {
			if !p.consumePrefix(")") {
				err = p.errorf("unclosed (")
			}
		}
		return v, err
	} else if p.consumePrefixIn("0x", "0X") != "" {
		// hexadecimal number
		s := p.consumeWhileIn(hexadecimalDigitsSet)
		if s == "" {
			return 0, p.errorf("empty hexadecimal literal")
		}
		return strconv.ParseInt(s, 16, 64)
	} else if p.consumePrefix("0") {
		// octal number
		s := p.consumeWhileIn(octalDigitsSet)
		if s == "" {
			return 0, p.errorf("empty octal literal")
		}
		return strconv.ParseInt(s, 8, 64)
	} else if s := p.consumeWhileIn(decimalDigitsSet); s != "" {
		// decimal number
		return strconv.ParseInt(s, 10, 64)
	} else if p.consumePrefix("~") {
		f, err := p.factor()
		return ^f, err
	} else if p.consumePrefix("!") {
		f, err := p.factor()
		return neg(f), err
	} else {
		return 0, p.errorf("can't parse a factor")
	}
}

func neg(i int64) int64 {
	if i == 0 {
		return 1
	}
	return 0
}
