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
	left := p.consumeSet(digitSet)
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

// Compound = [ TildePrefix ] { Primary }
type Compound struct {
	node
	TildePrefix string
	Parts       []*Primary
}

func (cp *Compound) parseInner(p *parser) {
	// TODO: Parse TildePrefix correctly in assignment RHS
	if prefix := findTildePrefix(p.rest()); prefix != "" {
		p.consume(len(prefix))
		cp.TildePrefix = prefix
	}
	for p.mayParseExpr() {
		p.parseInto(&cp.Parts, &Primary{})
	}
}

func findTildePrefix(s string) string {
	if !hasPrefix(s, "~") {
		return ""
	}
	for i, r := range s {
		if i == 0 {
			continue
		}
		if r == '/' || runeIn(r, exprStopper) {
			return s[:i]
		} else if runeIn(r, barewordStopper) {
			return ""
		}
	}
	return s
}

// Primary = Bareword
type Primary struct {
	node
	Type     PrimaryType
	Value    string
	Variable *Variable
}

type PrimaryType int

const (
	InvalidPrimary PrimaryType = iota
	BarewordPrimary
	VariablePrimary
	SingleQuotedPrimary
)

func (pr *Primary) parseInner(p *parser) {
start:
	switch {
	case p.nextInCompl(barewordStopper):
		pr.Type = BarewordPrimary
		pr.Value = p.consumeComplSet(barewordStopper)
	case p.consumePrefix("'"):
		pr.Type = SingleQuotedPrimary
		pr.Value = p.consumeComplSet("'")
		if !p.consumePrefix("'") {
			p.errorf("unterminated single-quoted string")
		}
	case p.consumePrefix("$"):
		pr.Type = VariablePrimary
		p.parseInto(&pr.Variable, &Variable{})
	case p.eof():
		p.errorf("EOF where an expression is expected")
	default:
		p.skipInvalid()
		fmt.Println("skipped one char, restart primary")
		goto start
	}
}

type Variable struct {
	node
	Name      string
	LengthOp  bool
	Modifiers *Modifier
}

var (
	specialVariableSet = "@*#?-$!"
	letterSet          = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	nameSet            = "_" + digitSet + letterSet
)

func (va *Variable) parseInner(p *parser) {
	if !p.consumePrefix("{") {
		// No braces, e.g. $x
		va.Name = parseVariableName(p, false)
		return
	}
	// Variable with braces, e.g. ${x:-fallback}
	if p.consumePrefix("#") {
		// We have seen "${#". It can either be the variable $# or the string
		// length operator, depending on what comes next.
		if p.startsWithOneOf("-}", "?}", "=}", "+}") != "" {
			// This is ambiguous, but POSIX prefers # to be parsed as the string
			// length operator. Note that since $=
			// and $+ are not valid variable names this will eventually result
			// in a parse error. We permit those.
			va.LengthOp = true
			va.Name = p.consume(1)
			// va.Name = parseVariableName(p, true)
		} else if p.startsWithOneOf("=}", "+}") != "" {
			// According to POSIX, "${#=}" and "${#+}" should also parse the #
			// as the string length operator. However, since $= and $+ are not
			// valid variable names, this will result in an error. Parsing them
			// as modifiers with an empty argument doesn't make sense either,
			// since $# cannot be assigned and is always set. We complain and
			// treat them as ${#}.
			p.errorf("invalid parameter substitution ${#%s}, treating as ${#}", p.consume(1))
			va.Name = "#"
		} else if p.startsWith("}") || p.startsWithOneOf(modifierOps...) != "" {
			va.Name = "#"
		} else {
			va.LengthOp = true
			va.Name = parseVariableName(p, true)
		}
	} else {
		va.Name = parseVariableName(p, true)
	}
	if p.startsWithCompl("}") {
		p.parseInto(&va.Modifiers, &Modifier{})
	}
	p.mustConsumePrefix("}")
	return
}

func parseVariableName(p *parser, brace bool) string {
	if name := p.consumeOneOfSet(specialVariableSet); name != "" {
		// Name may be one of the special variables. In that case, the name
		// is always just one character. For instance, $$x is the same as $$"x",
		// and ${$x} is invalid.
		return name
	} else if name0 := p.consumeOneOfSet(digitSet); name0 != "" {
		// Name starts with a digit. If the variable is braced, the name can be
		// a run of digits; otherwise the name is one digit. For instance, $0x
		// is the same as $0"x", and ${0x} is invalid; $01 is the same as $0"1",
		// and ${01} is the same as $1.
		if !brace {
			return name0
		}
		return name + p.consumeSet(digitSet)
	} else if name := p.consumeSet(nameSet); name != "" {
		// Parse an ordinary variable name, a run of characters in nameSet and
		// not starting with a digit. We already know that the name won't start
		// with a digit because that case is handled by the previous branch.
		return name
	} else {
		p.errorf("missing or invalid variable name, assuming '_'")
		return "_"
	}
}

type Modifier struct {
	node
	Operator string
	Argument *Compound
}

var modifierOps = []string{
	":-", "-", ":=", "=", ":?", "?", ":+", "+", "%%", "%", "##", "#",
}

func (md *Modifier) parseInner(p *parser) {
	md.Operator = p.consumeOneOf(modifierOps...)
	if md.Operator == "" {
		p.errorf("missing or invalid variable modifier, assuming ':-'")
		md.Operator = ":-"
	}
	p.parseInto(&md.Argument, &Compound{})
}

// Lookahead.

var (
	commandStopper  = " \t\r\n;()}&|"
	exprStopper     = commandStopper + "<>"
	barewordStopper = exprStopper + `[]{"'$*?`
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
