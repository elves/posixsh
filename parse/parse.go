// Package parse implements parsing of POSIX shell scripts.
package parse

//go:generate stringer -type=RedirMode,PrimaryType,DQSegmentType -output=string.go

import (
	"bytes"
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

var commandStopper = " \t\r\n;()}&|"

// Chunk = sw { Pipeline sw }
func (ch *Chunk) parseInner(p *parser) {
	p.sw()
	for p.mayParseCommand() {
		p.parseInto(&ch.Pipelines, &Pipeline{})
		p.sw()
	}
}

type Pipeline struct {
	node
	Forms []*Form
}

// Pipeline = Form iw { "|" w Form iw }
func (pp *Pipeline) parseInner(p *parser) {
	p.parseInto(&pp.Forms, &Form{})
	p.iw()
	for p.maybeMeta("|") {
		p.w()
		p.parseInto(&pp.Forms, &Form{})
		p.iw()
	}
}

type Form struct {
	node
	Words  []*Compound
	Redirs []*Redir
	FnBody *CompoundCommand // If non-nil, this is a function definition form.
}

const digitSet = "0123456789"

// Form = w { ( Redir | Compound ) iw } [ '(' iw ')' CompoundCommand ]
func (fm *Form) parseInner(p *parser) {
	p.w()
items:
	for {
		restPastDigits := strings.TrimLeft(p.rest(), digitSet)
		switch {
		case hasPrefix(restPastDigits, "<"), hasPrefix(restPastDigits, ">"):
			p.parseInto(&fm.Redirs, &Redir{})
		case p.mayParseExpr():
			p.parseInto(&fm.Words, &Compound{})
		default:
			break items
		}
		p.iw()
	}
	if p.maybeMeta("(") {
		// Parse a function definition.
		p.iw()
		p.meta(")")
		p.parseInto(&fm.FnBody, &CompoundCommand{})
	}
}

type Heredoc struct {
	node
	delim string
	Value string
}

// This function is called in (*Whitespaces).parseInner immediately after a \n,
// for each pending Heredoc.
func (hd *Heredoc) parseInner(p *parser) {
	begin := p.pos
	for i := p.pos; i < len(p.text); {
		j := i + strings.IndexByte(p.text[i:], '\n')
		iNext := j + 1
		if j == -1 {
			j = len(p.text)
			iNext = j
		}
		line := p.text[i:j]
		if line == hd.delim {
			hd.Value = p.text[begin:i]
			p.pos = iNext
			return
		}
		i = iNext
	}
	p.errorf("undelimited heredoc %q", hd.delim)
	hd.Value = p.text[begin:]
	p.pos = len(p.text)
}

type Redir struct {
	node
	Left    int // -1 for absense
	Mode    RedirMode
	RightFd bool
	Right   *Compound
	Heredoc *Heredoc
}

type RedirMode int

const (
	RedirInvalid RedirMode = iota
	RedirInput
	RedirOutput
	RedirInputOutput
	RedirAppend
	RedirHeredoc
)

// Redir = `[0-9]*` (">>" | "<>" | ">" | "<" | "<<") w [ "&" w ] Compound
func (rd *Redir) parseInner(p *parser) {
	left := p.consumeWhileIn(digitSet)
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
	case p.maybeMeta("<<"):
		rd.Mode = RedirHeredoc
	case p.maybeMeta(">"):
		rd.Mode = RedirOutput
	case p.maybeMeta("<"):
		rd.Mode = RedirInput
	default:
		p.errorf("missing redirection symbol, assuming <")
		rd.Mode = RedirInput
	}
	p.w()
	if p.maybeMeta("&") {
		if rd.Mode == RedirHeredoc {
			p.errorf("<<& is not allowed, ignoring &")
		} else {
			rd.RightFd = true
		}
		p.w()
	}
	p.parseInto(&rd.Right, &Compound{})
	if rd.Mode == RedirHeredoc {
		rd.Heredoc = &Heredoc{delim: p.text[rd.Right.Begin():rd.Right.End()]}
		p.pendingHeredocs = append(p.pendingHeredocs, rd.Heredoc)
	}
}

type CompoundCommand struct {
	node
	Subshell bool
	Body     *Chunk
}

// CompoundCommand = w '{' Chunk w '}'
//                 | w '(' Chunk w ')'
func (cc *CompoundCommand) parseInner(p *parser) {
	p.w()
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

var exprStopper = commandStopper + "<>"

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
	Type PrimaryType
	// String value. Valid for BarewordPrimary, SingleQuotedPrimary and
	// WildcardCharPrimary. For the first two types, the value contains the
	// processed value, e.g. the bareword \a has value "a".
	Value      string
	Variable   *Variable
	DQSegments []*DQSegment
}

type PrimaryType int

const (
	InvalidPrimary PrimaryType = iota
	BarewordPrimary
	VariablePrimary
	SingleQuotedPrimary
	DoubleQuotedPrimary
	WildcardCharPrimary
)

var (
	barewordStopper    = exprStopper + "'\"$[]?*{"
	rawBarewordStopper = barewordStopper + "\\"
)

func (pr *Primary) parseInner(p *parser) {
start:
	switch {
	case p.nextInCompl(barewordStopper):
		pr.Type = BarewordPrimary
		// Optimization: Consume a prefix that does not contain backslashes.
		// This avoid building a bytes.Buffer when the bareword is free of
		// backslashes.
		raw := p.consumeWhileNotIn(rawBarewordStopper)
		if !p.hasPrefix("\\") {
			// One of barewordStopper runes or EOF was encounterd.
			pr.Value = raw
			return
		}
		buf := bytes.NewBufferString(raw)
		lastBackslash := false
		p.consumeWhile(func(r rune) bool {
			if lastBackslash {
				buf.WriteRune(r)
				lastBackslash = false
				return true
			} else if r == '\\' {
				lastBackslash = true
				return true
			} else if runeIn(r, barewordStopper) {
				return false
			} else {
				buf.WriteRune(r)
				return true
			}
		})
		pr.Value = buf.String()
	case p.consumePrefix("'"):
		pr.Type = SingleQuotedPrimary
		begin := p.pos
		_ = p.consumeWhileNotIn("'")
		end := p.pos
		// recoverPos returns a postion after all line continuations. When the
		// single-quoted string has leading line continuations, those will be
		// skipped. Hence, we adjust begin to the position of the opening quote,
		// and adjust it back after recovery.
		pr.Value = p.orig[p.recoverPos(begin-1)+1 : p.recoverPos(end)]
		if !p.consumePrefix("'") {
			p.errorf("unterminated single-quoted string")
		}
	case p.consumePrefix(`"`):
		pr.Type = DoubleQuotedPrimary
		for !p.consumePrefix(`"`) {
			p.parseInto(&pr.DQSegments, &DQSegment{})
		}
	case p.consumePrefix("$"):
		pr.Type = VariablePrimary
		p.parseInto(&pr.Variable, &Variable{})
	case p.consumeRuneIn("[]*?") != "":
		pr.Type = WildcardCharPrimary
	case p.eof():
		p.errorf("EOF where an expression is expected")
	default:
		p.skipInvalid()
		fmt.Println("skipped one char, restart primary")
		goto start
	}
}

type DQSegment struct {
	node
	Type      DQSegmentType
	Value     string
	Expansion *Primary
}

type DQSegmentType int

const (
	DQInvalidSegment DQSegmentType = iota
	DQStringSegment
	DQExpansionSegment
)

var (
	dqStringSegmentStopper    = "$`\""
	rawDQStringSegmentStopper = dqStringSegmentStopper + "\\"
)

func (dq *DQSegment) parseInner(p *parser) {
	if p.hasPrefixIn("$", "`") != "" {
		p.parseInto(&dq.Expansion, &Primary{})
	} else {
		// Optimization: Consume a prefix that does not contain backslashes.
		// This avoid building a bytes.Buffer when this segment is free of
		// backslashes.
		raw := p.consumeWhileNotIn(rawDQStringSegmentStopper)
		if !p.hasPrefix("\\") {
			dq.Value = raw
			return
		}
		buf := bytes.NewBufferString(raw)
		lastBackslash := false
		p.consumeWhile(func(r rune) bool {
			if lastBackslash {
				if !runeIn(r, rawDQStringSegmentStopper) {
					buf.WriteRune('\\')
				}
				buf.WriteRune(r)
				lastBackslash = false
				return true
			} else if r == '\\' {
				lastBackslash = true
				return true
			} else if runeIn(r, dqStringSegmentStopper) {
				return false
			} else {
				buf.WriteRune(r)
				return true
			}
		})
		dq.Value = buf.String()
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
		if p.hasPrefixIn("-}", "?}", "=}", "+}") != "" {
			// This is ambiguous, but POSIX prefers # to be parsed as the string
			// length operator. Note that since $=
			// and $+ are not valid variable names this will eventually result
			// in a parse error. We permit those.
			va.LengthOp = true
			va.Name = p.consume(1)
			// va.Name = parseVariableName(p, true)
		} else if p.hasPrefixIn("=}", "+}") != "" {
			// According to POSIX, "${#=}" and "${#+}" should also parse the #
			// as the string length operator. However, since $= and $+ are not
			// valid variable names, this will result in an error. Parsing them
			// as modifiers with an empty argument doesn't make sense either,
			// since $# cannot be assigned and is always set. We complain and
			// treat them as ${#}.
			p.errorf("invalid parameter substitution ${#%s}, treating as ${#}", p.consume(1))
			va.Name = "#"
		} else if p.hasPrefix("}") || p.hasPrefixIn(modifierOps...) != "" {
			va.Name = "#"
		} else {
			va.LengthOp = true
			va.Name = parseVariableName(p, true)
		}
	} else {
		va.Name = parseVariableName(p, true)
	}
	if p.hasPrefixNot("}") {
		p.parseInto(&va.Modifiers, &Modifier{})
	}
	if !p.consumePrefix("}") {
		p.errorf("missing } to match {")
	}
	return
}

func parseVariableName(p *parser, brace bool) string {
	if name := p.consumeRuneIn(specialVariableSet); name != "" {
		// Name may be one of the special variables. In that case, the name
		// is always just one character. For instance, $$x is the same as $$"x",
		// and ${$x} is invalid.
		return name
	} else if name0 := p.consumeRuneIn(digitSet); name0 != "" {
		// Name starts with a digit. If the variable is braced, the name can be
		// a run of digits; otherwise the name is one digit. For instance, $0x
		// is the same as $0"x", and ${0x} is invalid; $01 is the same as $0"1",
		// and ${01} is the same as $1.
		if !brace {
			return name0
		}
		return name + p.consumeWhileIn(digitSet)
	} else if name := p.consumeWhileIn(nameSet); name != "" {
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
	md.Operator = p.consumePrefixIn(modifierOps...)
	if md.Operator == "" {
		p.errorf("missing or invalid variable modifier, assuming ':-'")
		md.Operator = ":-"
	}
	p.parseInto(&md.Argument, &Compound{})
}

// Lookahead.

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
