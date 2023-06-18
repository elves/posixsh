package parse

// Basic types used by the package.

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

type parser struct {
	orig string
	// Text with \<newline> removed.
	text string
	// Occurrences of line continuations, as indices into text. This is useful
	// when recovering the real position when reporting error or parsing
	// single-quoted strings ( which is the only place where \<newline> does not
	// function as line continuation).
	lineCont []int

	pos   int
	stack []Node
	err   Error
	// Heredocs are collected into this list when parsing the leader (e.g.
	// <<EOF), and resolved when parsing newlines.
	pendingHeredocs []*Heredoc
}

func newParser(orig string) *parser {
	var lineCont []int
	buf := &bytes.Buffer{}

	lastBackslash := false
	for _, r := range orig {
		if lastBackslash {
			if r == '\n' {
				lineCont = append(lineCont, buf.Len())
			} else {
				buf.WriteRune('\\')
				buf.WriteRune(r)
			}
			lastBackslash = false
		} else if r == '\\' {
			lastBackslash = true
		} else {
			buf.WriteRune(r)
		}
	}
	// NOTE: \ just before EOF is treated as a line continuation.
	if lastBackslash {
		lineCont = append(lineCont, buf.Len())
	}
	return &parser{orig: orig, text: buf.String(), lineCont: lineCont}
}

func (p *parser) recoverPos(pos int) int {
	// sort.SearchInts(a, i+1) returns the number of elements in a that <= i.
	// Here, we find the number of line continuations that occur before pos
	// (inclusive). Each line continuation occupies two bytes.
	return pos + 2*sort.SearchInts(p.lineCont, pos+1)
}

func (p *parser) rest() string {
	return p.text[p.pos:]
}

func (p *parser) eof() bool {
	return p.rest() == ""
}

func (p *parser) source(n Node) string {
	return p.text[n.Begin():n.End()]
}

func (p *parser) errorf(format string, a ...interface{}) {
	p.err.Errors = append(p.err.Errors,
		ErrorEntry{p.recoverPos(p.pos), fmt.Sprintf(format, a...)})
}

func (p *parser) consume(i int) string {
	consumed := p.rest()[:i]
	p.pos += i
	return consumed
}

func (p *parser) consumeWhile(f func(r rune) bool) string {
	for i, r := range p.rest() {
		if !f(r) {
			return p.consume(i)
		}
	}
	return p.consume(len(p.rest()))
}

func (p *parser) consumeWhileIn(set string) string {
	return p.consumeWhile(func(r rune) bool { return runeIn(r, set) })
}

func (p *parser) consumeWhileNotIn(set string) string {
	return p.consumeWhile(func(r rune) bool { return !runeIn(r, set) })
}

func (p *parser) hasPrefix(prefix string) bool {
	return hasPrefix(p.rest(), prefix)
}

func (p *parser) hasPrefixNot(prefix string) bool {
	return p.rest() != "" && !hasPrefix(p.rest(), prefix)
}

func (p *parser) hasPrefixIn(prefixes ...string) string {
	for _, prefix := range prefixes {
		if p.hasPrefix(prefix) {
			return prefix
		}
	}
	return ""
}

func (p *parser) consumePrefix(prefix string) bool {
	return p.consumePrefixIn(prefix) == prefix
}

func (p *parser) consumePrefixIn(prefixes ...string) string {
	prefix := p.hasPrefixIn(prefixes...)
	p.consume(len(prefix))
	return prefix
}

func (p *parser) consumeRuneIn(set string) string {
	return p.consumePrefixIn(strings.Split(set, "")...)
}

func (p *parser) skipInvalid() {
	r, size := utf8.DecodeRuneInString(p.rest())
	p.errorf("skipped invalid rune %q", r)
	p.consume(size)
}

// Common parsing logic.

func addTo[T any](ptr *[]T, v T) { *ptr = append(*ptr, v) }

type parseNode[O any] interface {
	Node
	parse(*parser, O)
}

func parse[O any, N parseNode[O]](p *parser, n N, opt O) N {
	n.setBegin(p.pos)
	p.stack = append(p.stack, n)

	n.parse(p, opt)

	n.setEnd(p.pos)
	p.stack[len(p.stack)-1] = nil
	p.stack = p.stack[:len(p.stack)-1]
	if len(p.stack) > 0 && !emptyWhitespaces(n) {
		parent := p.stack[len(p.stack)-1]
		parent.addChild(n)
		n.setParent(parent)
	}
	return n
}

func parseNoOpt[N parseNode[struct{}]](p *parser, n N) N {
	return parse(p, n, struct{}{})
}

func emptyWhitespaces(n Node) bool {
	w, ok := n.(*Whitespaces)
	return ok && w.begin == w.end
}

// Shorthands for parse calls.

func (p *parser) inlineWhitespace() {
	parseNoOpt(p, &InlineWhitespaces{})
}

func (p *parser) whitespace() {
	parse(p, &Whitespaces{}, whitespacesOpt(0))
}

func (p *parser) whitespaceOrSemicolon() {
	parse(p, &Whitespaces{}, semicolonIsWhitespace)
}

func (p *parser) meta(meta string) {
	parse(p, &Meta{}, meta)
}

func (p *parser) maybeMeta(meta string) bool {
	if p.hasPrefix(meta) {
		p.meta(meta)
		return true
	}
	return false
}
