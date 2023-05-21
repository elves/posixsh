package arith

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var whitespaceRegexp = regexp.MustCompile(`\s+`)

func Eval(s string, variables map[string]string) (int64, error) {
	s = whitespaceRegexp.ReplaceAllLiteralString(s, "")
	p := parser{basicParser{s, 0}, variables}
	result, err := p.expr()
	if err == nil && !p.eof() {
		// TODO: Add position
		err = p.errorf("trailing content: %v", p.rest())
	}
	return result, err
}

type parser struct {
	basicParser
	variables map[string]string
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

// POSIX doesn't specify whether special variables like $# should be supported
// in arithmetic expressions without the $, like "$(( # + 1))". Bash and dash
// don't; zsh does. We don't support them.
const varNameSet = "_0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

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
			// Just 0
			return 0, nil
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
		return not(f), err
	} else if name := p.consumeWhileIn(varNameSet); name != "" {
		// variable
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
	} else {
		return 0, p.errorf("can't parse a factor")
	}
}

// Parses a number. We don't use strconv.ParseInt(s, 0, 64) in order to ensure
// consistency with how literals are parsed.
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
