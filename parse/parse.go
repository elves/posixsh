// Package parse implements parsing of POSIX shell scripts.
package parse

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

func Parse(text string, n Node) (string, error) {
	p := newParser(text)
	p.parse(n)
	if len(p.err.Errors) == 0 {
		return p.rest(), nil
	}
	return p.rest(), p.err
}

type Chunk struct {
	node
	Pipelines []*Pipeline
}

// Chunk = w { Pipeline w }
func (ch *Chunk) parseInner(p *parser) {
	p.whitespaces()
	for p.mayParseCommand() {
		p.parseInto(&ch.Pipelines, &Pipeline{})
		p.whitespaces()
	}
}

type Pipeline struct {
	node
	Forms []*Form
}

// Pipeline = Form iw { "|" w Form iw }
func (pp *Pipeline) parseInner(p *parser) {
	p.parseInto(&pp.Forms, &Form{})
	p.inlineWhitespaces()
	for p.maybeMeta("|") {
		p.whitespaces()
		p.parseInto(&pp.Forms, &Form{})
		p.inlineWhitespaces()
	}
}

type Form struct {
	node
	Words  []*Compound
	Redirs []*Redir
	FnBody *CompoundCommand // If non-nil, this is a function definition form.
}

// Form = w { ( Compound | Redir ) iw } [ '(' iw ')' CompoundCommand ]
func (fm *Form) parseInner(p *parser) {
	p.whitespaces()
items:
	for {
		switch {
		case startsRedir(p.rest()):
			fmt.Println("parsing a redir")
			p.parseInto(&fm.Redirs, &Redir{})
		case p.mayParseExpr():
			p.parseInto(&fm.Words, &Compound{})
		default:
			break items
		}
		p.inlineWhitespaces()
	}
	if p.maybeMeta("(") {
		// Parse a function definition.
		p.inlineWhitespaces()
		p.meta(")")
		p.parseInto(&fm.FnBody, &CompoundCommand{})
	}
}

const digitSet = "0123456789"

func startsRedir(rest string) bool {
	rest = strings.TrimLeft(rest, digitSet)
	return rest != "" && (rest[0] == '<' || rest[0] == '>')
}

// Redir = w `[0-9]*` (">>" | "<>" | ">" | "<") w [ "&" w ] Compound
type Redir struct {
	node
	Left    int // -1 for absense
	Mode    RedirMode
	RightFd bool
	Right   *Compound
}

type RedirMode int

const (
	RedirInvalid RedirMode = iota
	RedirInput
	RedirOutput
	RedirInputOutput
	RedirAppend
)

func (rd *Redir) parseInner(p *parser) {
	p.whitespaces()
	left := p.advanceUntil(strings.TrimLeft(p.rest(), digitSet))
	if left == "" {
		rd.Left = -1
	} else {
		fd, err := strconv.Atoi(left)
		if err != nil {
			// Only possible when left is too long
			p.errorf("redir fd %s is too large, ignoring", left)
			rd.Left = -1
		} else {
			rd.Left = fd
		}
	}
	switch {
	case p.maybeMeta(">>"):
		rd.Mode = RedirAppend
	case p.maybeMeta("<>"):
		rd.Mode = RedirInputOutput
	case p.maybeMeta(">"):
		rd.Mode = RedirOutput
	case p.maybeMeta("<"):
		rd.Mode = RedirInput
	default:
		p.errorf("missing redirection symbol, assuming <")
		rd.Mode = RedirInput
	}
	p.whitespaces()
	if p.maybeMeta("&") {
		rd.RightFd = true
		p.whitespaces()
	}
	p.parseInto(&rd.Right, &Compound{})
}

type CompoundCommand struct {
	node
	Subshell bool
	Body     *Chunk
}

// CompoundCommand = w '{' Chunk w '}'
//                 | w '(' Chunk w ')'
func (cc *CompoundCommand) parseInner(p *parser) {
	p.whitespaces()
	closer := ""
	switch {
	case p.maybeMeta("("):
		closer = ")"
		cc.Subshell = true
	case p.maybeMeta("{"):
		closer = "}"
	default:
		p.errorf("missing '{' or '(' for compound command")
	}
	p.parseInto(&cc.Body, &Chunk{})
	if closer != "" {
		p.meta(closer)
	}
}

// Compound = { Primary }
type Compound struct {
	node
	Parts []*Primary
}

func (cp *Compound) parseInner(p *parser) {
	for p.mayParseExpr() {
		p.parseInto(&cp.Parts, &Primary{})
	}
}

// Primary = Bareword
type Primary struct {
	node
	Type  PrimaryType
	Value string
}

type PrimaryType int

const (
	Invalid PrimaryType = iota
	Bareword
	Variable
)

func (pr *Primary) parseInner(p *parser) {
start:
	switch {
	case p.nextInCompl(barewordStopper):
		pr.Type = Bareword
		pr.Value = p.consumeFunc(func(r rune) bool {
			return !runeIn(r, barewordStopper)
		})
	case p.maybeConsume("$"):
		pr.Type = Variable
		if p.maybeConsume("{") {
			pr.Value = p.consumeFunc(func(r rune) bool {
				return !runeIn(r, barewordStopper)
			})
			if !p.maybeConsume("}") {
				p.errorf("missing '}' to match '{'")
			}
		} else {
			pr.Value = p.consumeFunc(func(r rune) bool {
				return !runeIn(r, barewordStopper)
			})
		}
	default:
		p.skipInvalid()
		goto start
	}
}

// Lookahead.

var (
	commandStopper  = " \t\r\n;)&|"
	exprStopper     = commandStopper + "<>"
	barewordStopper = exprStopper + `([]{}"'$*?`
)

func (p *parser) mayParseCommand() bool {
	return p.nextInCompl(commandStopper)
}

func (p *parser) mayParseExpr() bool {
	return p.nextInCompl(exprStopper)
}

func (p *parser) nextIn(set string) bool {
	r, size := utf8.DecodeRuneInString(p.rest())
	return size > 0 && runeIn(r, set)
}

func (p *parser) nextInCompl(set string) bool {
	r, size := utf8.DecodeRuneInString(p.rest())
	return size > 0 && !runeIn(r, set)
}
