package arith

import (
	"strings"
)

type basicParser struct {
	text string
	pos  int
}

func (p *basicParser) rest() string {
	return p.text[p.pos:]
}

func (p *basicParser) eof() bool {
	return p.rest() == ""
}
func (p *basicParser) consume(i int) string {
	consumed := p.rest()[:i]
	p.pos += i
	return consumed
}

func (p *basicParser) consumeWhile(f func(r rune) bool) string {
	for i, r := range p.rest() {
		if !f(r) {
			return p.consume(i)
		}
	}
	return p.consume(len(p.rest()))
}

func (p *basicParser) consumeWhileIn(set string) string {
	return p.consumeWhile(func(r rune) bool { return strings.ContainsRune(set, r) })
}

func (p *basicParser) consumeWhileNotIn(set string) string {
	return p.consumeWhile(func(r rune) bool { return !strings.ContainsRune(set, r) })
}

func (p *basicParser) hasPrefix(prefix string) bool {
	return strings.HasPrefix(p.rest(), prefix)
}

func (p *basicParser) hasPrefixNot(prefix string) bool {
	return p.rest() != "" && !strings.HasPrefix(p.rest(), prefix)
}

func (p *basicParser) hasPrefixIn(prefixes ...string) string {
	for _, prefix := range prefixes {
		if p.hasPrefix(prefix) {
			return prefix
		}
	}
	return ""
}

func (p *basicParser) consumePrefix(prefix string) bool {
	return p.consumePrefixIn(prefix) == prefix
}

func (p *basicParser) consumePrefixIn(prefixes ...string) string {
	prefix := p.hasPrefixIn(prefixes...)
	p.consume(len(prefix))
	return prefix
}

func (p *basicParser) consumeRuneIn(set string) string {
	return p.consumePrefixIn(strings.Split(set, "")...)
}
