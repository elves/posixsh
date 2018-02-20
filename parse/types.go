package parse

// Basic types used by the package.

import (
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

type parser struct {
	text  string
	pos   int
	stack []Node
	err   Error
}

func newParser(text string) *parser {
	return &parser{text: text}
}

func (p *parser) parse(n Node) {
	fmt.Printf("parse %T, pos %d\n", n, p.pos)

	n.setBegin(p.pos)
	p.stack = append(p.stack, n)

	n.parseInner(p)

	n.setEnd(p.pos)
	p.stack[len(p.stack)-1] = nil
	p.stack = p.stack[:len(p.stack)-1]
	if len(p.stack) > 0 {
		parent := p.stack[len(p.stack)-1]
		parent.addChild(n)
		n.setParent(parent)
	}
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

func (p *parser) errorf(format string, a ...interface{}) {
	p.err.Errors = append(p.err.Errors,
		ErrorEntry{p.pos, fmt.Sprintf(format, a...)})
}

func (p *parser) rest() string {
	return p.text[p.pos:]
}

func (p *parser) advance(i int) {
	p.pos += i
}

func (p *parser) skipInvalid() {
	r, size := utf8.DecodeRuneInString(p.rest())
	p.errorf("skipped invalid rune %q", r)
	p.advance(size)
}

func (p *parser) advanceUntil(newRest string) string {
	rest := p.rest()
	consumed := rest[:len(rest)-len(newRest)]
	p.advance(len(consumed))
	return consumed
}

func (p *parser) consumeSet(set string) string {
	return p.advanceUntil(strings.TrimLeft(p.rest(), set))
}

func (p *parser) consumeFunc(f func(r rune) bool) string {
	return p.advanceUntil(strings.TrimLeftFunc(p.rest(), f))
}

func (p *parser) maybeConsume(prefix string) bool {
	if strings.HasPrefix(p.rest(), prefix) {
		p.advance(len(prefix))
		return true
	}
	return false
}

// Shorthands for parsing whitespace nodes.

func (p *parser) inlineWhitespaces() {
	p.parse(&Whitespaces{set: inlineWhitespaceSet})
}

func (p *parser) whitespaces() {
	p.parse(&Whitespaces{set: whitespaceSet})
}

func (p *parser) meta(meta string) {
	p.parse(&Meta{meta: meta})
}

func (p *parser) maybeMeta(meta string) bool {
	if strings.HasPrefix(p.rest(), meta) {
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
	set string
}

func (ws *Whitespaces) parseInner(p *parser) {
	comment := false
	p.consumeFunc(func(r rune) bool {
		if r == '#' {
			comment = true
		} else if r == '\n' {
			comment = false
		}
		return comment || runeIn(r, ws.set)
	})
}

type Meta struct {
	node
	meta string
}

func (mt *Meta) parseInner(p *parser) {
	if strings.HasPrefix(p.rest(), mt.meta) {
		p.advance(len(mt.meta))
	} else {
		p.errorf("missing meta symbol %q", mt.meta)
	}
}

func runeIn(r rune, set string) bool {
	return strings.ContainsRune(set, r)
}
