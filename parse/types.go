package parse

// Basic types used by the package.

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"
)

type parser struct {
	orig string
	// Text with \<newline> removed.
	text string
	// Occurances of line continuations, as indicies into text. This is useful
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

func (p *parser) parse(n Node) {
	fmt.Printf("parse %T, pos %d\n", n, p.pos)

	n.setBegin(p.pos)
	p.stack = append(p.stack, n)

	n.parseInner(p)

	n.setEnd(p.pos)
	p.stack[len(p.stack)-1] = nil
	p.stack = p.stack[:len(p.stack)-1]
	if len(p.stack) > 0 && !emptyWhitespaces(n) {
		parent := p.stack[len(p.stack)-1]
		parent.addChild(n)
		n.setParent(parent)
	}
}

func emptyWhitespaces(n Node) bool {
	w, ok := n.(*Whitespaces)
	return ok && w.begin == w.end
}

func (p *parser) parseInto(ptr interface{}, n Node) {
	p.parse(n)
	dst := reflect.ValueOf(ptr).Elem()
	if dst.Type().AssignableTo(nodeTyp) {
		// Assume that ptr is of type *T, where T < Node
		dst.Set(reflect.ValueOf(n))
	} else {
		// Assume that ptr is of type *[]T, where T < Node
		dst.Set(reflect.Append(dst, reflect.ValueOf(n)))
	}
}

// Shorthands for .parse calls.

func (p *parser) iw() {
	p.parse(&Whitespaces{inline: true})
}

func (p *parser) w() {
	p.parse(&Whitespaces{})
}

func (p *parser) meta(meta string) {
	p.parse(&Meta{meta: meta})
}

func (p *parser) maybeMeta(meta string) bool {
	if p.hasPrefix(meta) {
		p.meta(meta)
		return true
	}
	return false
}

// Parse error.

type Error struct {
	Errors []ErrorEntry
}

func (err Error) Error() string {
	// TODO
	return fmt.Sprintf("%d parse errors", len(err.Errors))
}

type ErrorEntry struct {
	Position int
	Message  string
}

type Node interface {
	Begin() int
	End() int
	Parent() Node
	Children() []Node

	setBegin(int)
	setEnd(int)
	setParent(Node)
	addChild(Node)
	parseInner(*parser)
}

type node struct {
	begin    int
	end      int
	parent   Node
	children []Node
}

var nodeTyp = reflect.TypeOf((*Node)(nil)).Elem()

func (n *node) Begin() int       { return n.begin }
func (n *node) setBegin(i int)   { n.begin = i }
func (n *node) End() int         { return n.end }
func (n *node) setEnd(i int)     { n.end = i }
func (n *node) Parent() Node     { return n.parent }
func (n *node) setParent(m Node) { n.parent = m }
func (n *node) Children() []Node { return n.children }
func (n *node) addChild(m Node)  { n.children = append(n.children, m) }

const (
	inlineWhitespaceSet = " \t\r"
	whitespaceSet       = inlineWhitespaceSet + "\n"
)

// Whitespaces is a leaf Node made up of a run of zero or more whitespace
// characters.
type Whitespaces struct {
	node
	inline bool
}

func (w *Whitespaces) parseInner(p *parser) {
	consumeWhitespacesAndComment(p, inlineWhitespaceSet)
	if w.inline {
		return
	}
	for _, pending := range p.pendingHeredocs {
		p.parse(pending)
	}
	p.pendingHeredocs = nil
	consumeWhitespacesAndComment(p, whitespaceSet)
}

func consumeWhitespacesAndComment(p *parser, set string) {
	comment := false
	p.consumeWhile(func(r rune) bool {
		if r == '#' {
			comment = true
		} else if r == '\n' {
			comment = false
		}
		return comment || runeIn(r, set)
	})
}

type Meta struct {
	node
	meta string
}

func (mt *Meta) parseInner(p *parser) {
	if p.hasPrefix(mt.meta) {
		p.consume(len(mt.meta))
	} else {
		p.errorf("missing meta symbol %q", mt.meta)
	}
}

func runeIn(r rune, set string) bool {
	return strings.ContainsRune(set, r)
}
