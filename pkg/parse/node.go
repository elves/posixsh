package parse

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
	inline    bool
	semicolon bool
}

func (w *Whitespaces) parseInner(p *parser) {
	consumeWhitespacesAndComment(p, inlineWhitespaceSet, w.semicolon)
	if w.inline {
		return
	}
	if !p.consumePrefix("\n") {
		return
	}
	for _, pending := range p.pendingHeredocs {
		parse(p, pending)
	}
	p.pendingHeredocs = nil
	consumeWhitespacesAndComment(p, whitespaceSet, w.semicolon)
}

func consumeWhitespacesAndComment(p *parser, set string, semicolon bool) {
	if semicolon {
		set += ";"
	}
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
