package parse

type Node interface {
	Begin() int
	End() int
	Parent() Node
	Children() []Node
	Source() string

	setBegin(int)
	setEnd(int)
	setSource(string)
	setParent(Node)
	addChild(Node)
}

type node struct {
	begin    int
	end      int
	source   string
	parent   Node
	children []Node
}

func (n *node) Begin() int         { return n.begin }
func (n *node) setBegin(i int)     { n.begin = i }
func (n *node) End() int           { return n.end }
func (n *node) setEnd(i int)       { n.end = i }
func (n *node) Source() string     { return n.source }
func (n *node) setSource(s string) { n.source = s }
func (n *node) Parent() Node       { return n.parent }
func (n *node) setParent(m Node)   { n.parent = m }
func (n *node) Children() []Node   { return n.children }
func (n *node) addChild(m Node)    { n.children = append(n.children, m) }

const (
	inlineWhitespaceSet = " \t\r"
	whitespaceSet       = inlineWhitespaceSet + "\n"
)

// InlineWhitespaces is a leaf Node made up of a run of zero or more inline
// whitespace characters.
type InlineWhitespaces struct{ node }

func (iw *InlineWhitespaces) parse(p *parser, _ struct{}) {
	consumeWhitespacesAndComment(p, inlineWhitespaceSet, false)
}

// Whitespaces is a leaf Node made up of a run of zero or more whitespace
// characters.
type Whitespaces struct{ node }

type whitespacesOpt uint

const (
	semicolonIsWhitespace whitespacesOpt = 1 << iota
)

func (w *Whitespaces) parse(p *parser, opt whitespacesOpt) {
	semicolon := opt&semicolonIsWhitespace != 0
	consumeWhitespacesAndComment(p, inlineWhitespaceSet, semicolon)
	if !p.consumePrefix("\n") {
		return
	}
	for _, pending := range p.pendingHeredocs {
		parse(p, pending.dest, pending)
	}
	p.pendingHeredocs = nil
	consumeWhitespacesAndComment(p, whitespaceSet, semicolon)
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

type Meta struct{ node }

func (mt *Meta) parse(p *parser, meta string) {
	if p.hasPrefix(meta) {
		p.consume(len(meta))
	} else {
		p.errorf("missing meta symbol %q", meta)
	}
}
