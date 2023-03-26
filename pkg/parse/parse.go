// Package parseNoOpt implements parsing of POSIX shell scripts.
package parse

//go:generate stringer -type=RedirMode,PrimaryType,DQSegmentType -output=string.go

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

func Parse(text string) (*Chunk, error) {
	p := newParser(text)
	n := &Chunk{}
	parse(p, n, nodeOpt(0))
	if p.rest() != "" {
		p.errorf("unparsed code")
	}
	if len(p.err.Errors) == 0 {
		return n, nil
	}
	return n, p.err
}

type Chunk struct {
	node
	AndOrs []*AndOr
}

var commandStopper = " ;)}&|"

type nodeOpt uint

const (
	// In backquotes, and not in a bracketed construct within.
	inBackquotes nodeOpt = 1 << iota
)

// Chunk = sw { AndOr sw }
func (ch *Chunk) parse(p *parser, opt nodeOpt) {
	p.whitespaceOrSemicolon()
	for p.mayParseCommand(opt) {
		addTo(&ch.AndOrs, parse(p, &AndOr{}, opt))
		p.whitespaceOrSemicolon()
	}
}

type AndOr struct {
	node
	Pipelines []*Pipeline
	AndOp     []bool
}

// AndOr = Pipeline iw { ("&&" | "||") w Pipeline iw }
func (ao *AndOr) parse(p *parser, opt nodeOpt) {
	addTo(&ao.Pipelines, parse(p, &Pipeline{}, opt))
	p.inlineWhitespace()
	for {
		// NOTE: Should be meta
		op := p.consumePrefixIn("&&", "||")
		if op == "" {
			break
		}
		ao.AndOp = append(ao.AndOp, op == "&&")
		p.whitespace()
		addTo(&ao.Pipelines, parse(p, &Pipeline{}, opt))
		p.inlineWhitespace()
	}
}

type Pipeline struct {
	node
	Forms []*Form
}

// Pipeline = Form iw { ("|" \ "||") w Form iw }
func (pp *Pipeline) parse(p *parser, opt nodeOpt) {
	addTo(&pp.Forms, parse(p, &Form{}, opt))
	p.inlineWhitespace()
	for p.hasPrefix("|") && !p.hasPrefix("||") {
		// | should be meta
		p.consumePrefix("|")
		p.whitespace()
		addTo(&pp.Forms, parse(p, &Form{}, opt))
		p.inlineWhitespace()
	}
}

type Form struct {
	node
	Type    FormType
	Assigns []*Assign
	Words   []*Compound
	Redirs  []*Redir
	Body    *CompoundCommand // Non-nil for FnDefinitionForm and CompoundCommandForm
}

type FormType int

const (
	InvalidForm FormType = iota
	NormalForm
	FnDefinitionForm
	CompoundCommandForm
)

const digitSet = "0123456789"

var assignPattern = regexp.MustCompile("^[a-zA-Z_][a-zA-Z_0-9]*=")

// Form = w CompoundCommand
//
//	| w { Assign iw } { ( Redir | Compound ) iw }
//	| w { Assign iw } { ( Redir | Compound ) iw } "(" iw ")" CompoundCommand
func (fm *Form) parse(p *parser, opt nodeOpt) {
	p.whitespace()
	if p.hasPrefixIn("(", "{") != "" {
		fm.Type = CompoundCommandForm
		fm.Body = parse(p, &CompoundCommand{}, opt)
		return
	}
	fm.Type = NormalForm
	if assignPattern.MatchString(p.rest()) {
		addTo(&fm.Assigns, parse(p, &Assign{}, opt))
		p.inlineWhitespace()
	}
items:
	for {
		restPastDigits := strings.TrimLeft(p.rest(), digitSet)
		switch {
		case hasPrefix(restPastDigits, "<"), hasPrefix(restPastDigits, ">"):
			addTo(&fm.Redirs, parse(p, &Redir{}, opt))
		case p.mayParseExpr(opt):
			addTo(&fm.Words, parse(p, &Compound{}, opt))
		default:
			break items
		}
		p.inlineWhitespace()
	}
	if p.maybeMeta("(") {
		fm.Type = FnDefinitionForm
		// Parse a function definition.
		p.inlineWhitespace()
		p.meta(")")
		fm.Body = parse(p, &CompoundCommand{}, opt)
	}
}

type Assign struct {
	node
	LHS string
	RHS *Compound
}

// Assign := `[a-zA-Z_][a-zA-Z0-9_]*` "=" Compound
func (as *Assign) parse(p *parser, opt nodeOpt) {
	s := assignPattern.FindString(p.rest())
	p.consume(len(s))
	if s == "" {
		p.errorf("missing LHS in assignment, assuming $_")
		as.LHS = "_"
	}
	as.LHS = s[:len(s)-1]
	as.RHS = parse(p, &Compound{}, opt)
}

type Heredoc struct {
	node
	delim  string
	quoted bool
	Value  string
}

// This function is called in (*Whitespaces).parseNoOpt immediately after a \n,
// for each pending Heredoc.
func (hd *Heredoc) parse(p *parser, _ struct{}) {
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
func (rd *Redir) parse(p *parser, opt nodeOpt) {
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
	p.whitespace()
	if p.maybeMeta("&") {
		if rd.Mode == RedirHeredoc {
			p.errorf("<<& is not allowed, ignoring &")
		} else {
			rd.RightFd = true
		}
		p.whitespace()
	}
	rd.Right = parse(p, &Compound{}, opt)
	if rd.Mode == RedirHeredoc {
		delim, quoted := parseHeredocDelim(p, rd.Right)
		rd.Heredoc = &Heredoc{delim: delim, quoted: quoted}
		p.pendingHeredocs = append(p.pendingHeredocs, rd.Heredoc)
	}
}

func parseHeredocDelim(p *parser, cp *Compound) (delim string, quoted bool) {
	var buf bytes.Buffer
	for _, pr := range cp.Parts {
		switch pr.Type {
		case SingleQuotedPrimary:
			quoted = true
			buf.WriteString(pr.Value)
		case DoubleQuotedPrimary:
			// TODO: Unquote properly
			quoted = true
			buf.WriteString(p.text[pr.Begin()+1 : pr.End()-1])
		default:
			buf.WriteString(p.text[pr.Begin():pr.End()])
		}
	}
	return buf.String(), quoted
}

type CompoundCommand struct {
	node
	Subshell bool
	Body     *Chunk
}

// CompoundCommand = w '{' Chunk w '}'
//
//	| w '(' Chunk w ')'
func (cc *CompoundCommand) parse(p *parser, opt nodeOpt) {
	p.whitespace()
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
	cc.Body = parse(p, &Chunk{}, opt&^inBackquotes)
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

var exprStopper = commandStopper + " \t\r\n<>("

func (cp *Compound) parse(p *parser, opt nodeOpt) {
	// TODO: Parse TildePrefix correctly in assignment RHS
	if prefix := findTildePrefix(p.rest()); prefix != "" {
		p.consume(len(prefix))
		cp.TildePrefix = prefix
	}
	for p.mayParseExpr(opt) {
		addTo(&cp.Parts, parse(p, &Primary{}, opt))
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
	Body       *Chunk // Valid for OutputCapturePrimary.
}

type PrimaryType int

const (
	InvalidPrimary PrimaryType = iota
	BarewordPrimary
	SingleQuotedPrimary
	DoubleQuotedPrimary
	WildcardCharPrimary
	OutputCapturePrimary
	VariablePrimary
)

var (
	barewordStopper    = exprStopper + "'\"$`[]?*{"
	rawBarewordStopper = barewordStopper + "\\"
)

func (pr *Primary) parse(p *parser, opt nodeOpt) {
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
		for !p.eof() && !p.consumePrefix(`"`) {
			addTo(&pr.DQSegments, parse(p, &DQSegment{}, opt))
		}
		if p.eof() {
			p.errorf("unterminated double-quoted string")
		}
	case p.consumeRuneIn("[]*?") != "":
		pr.Type = WildcardCharPrimary
	case p.consumePrefix("`"):
		pr.Type = OutputCapturePrimary
		pr.Body = parse(p, &Chunk{}, opt|inBackquotes)
		if !p.consumePrefix("`") {
			p.errorf("missing closing backquote for output capture")
		}
	case p.consumePrefix("$("):
		pr.Type = OutputCapturePrimary
		pr.Body = parse(p, &Chunk{}, opt|inBackquotes)
		if !p.consumePrefix(")") {
			p.errorf("missing closing paranthesis for output capture")
		}
	case p.consumePrefix("$"):
		if p.nextIn(variableInitialSet) {
			pr.Type = VariablePrimary
			pr.Variable = parse(p, &Variable{}, opt)
		} else {
			// If a variable can't be parsed, it's not an error but a bareword.
			pr.Type = BarewordPrimary
			pr.Value = "$"
		}
	case p.eof():
		p.errorf("EOF where an expression is expected")
	default:
		p.skipInvalid()
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

func (dq *DQSegment) parse(p *parser, opt nodeOpt) {
	if p.hasPrefixIn("$", "`") != "" {
		dq.Type = DQExpansionSegment
		dq.Expansion = parse(p, &Primary{}, opt&^inBackquotes)
	} else {
		dq.Type = DQStringSegment
		// Optimization: Consume a prefix that does not contain backslashes.
		// This avoids building a bytes.Buffer when this segment is free of
		// backslashes.
		raw := p.consumeWhileNotIn(rawDQStringSegmentStopper)
		if !p.hasPrefix("\\") {
			dq.Value = raw
			return
		}
		var b strings.Builder
		b.WriteString(raw)
		lastBackslash := false
		p.consumeWhile(func(r rune) bool {
			if lastBackslash {
				if !runeIn(r, rawDQStringSegmentStopper) {
					b.WriteRune('\\')
				}
				b.WriteRune(r)
				lastBackslash = false
				return true
			} else if r == '\\' {
				lastBackslash = true
				return true
			} else if runeIn(r, dqStringSegmentStopper) {
				return false
			} else {
				b.WriteRune(r)
				return true
			}
		})
		dq.Value = b.String()
	}
}

type Variable struct {
	node
	Name     string
	LengthOp bool
	Modifier *Modifier
}

var (
	variableInitialSet = "{" + specialVariableSet + nameSet
	specialVariableSet = "@*#?-$!"
	letterSet          = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	nameSet            = "_" + digitSet + letterSet
)

func (va *Variable) parse(p *parser, opt nodeOpt) {
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
			// in a parseNoOpt error. We permit those.
			va.LengthOp = true
			va.Name = p.consume(1)
			// va.Name = parseVariableName(p, true)
		} else if p.hasPrefixIn("=}", "+}") != "" {
			// According to POSIX, "${#=}" and "${#+}" should also parseNoOpt the #
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
		va.Modifier = parse(p, &Modifier{}, opt)
	}
	if !p.consumePrefix("}") {
		p.errorf("missing } to match {")
	}
}

// See https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_06_02.
func parseVariableName(p *parser, brace bool) string {
	if name := p.consumeRuneIn(specialVariableSet); name != "" {
		// Name may be one of the special variables. In that case, the name is
		// always just one character, even inside braces. POSIX seems to have
		// only defined the case when there are braces.
		//
		// For instance, $$x is the same as $$"x", and ${$x} is invalid.
		return name
	} else if brace {
		// Consume a run of characters in nameSet, including digits.
		if name := p.consumeWhileIn(nameSet); name != "" {
			return name
		}
		p.errorf("missing or invalid variable name, assuming '_'")
		return "_"
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
		// If we have reached this point, the variable name doesn't have braces
		// and doesn't start with a digit. Consume a run of characters in
		// nameSet, including digits.
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

func (md *Modifier) parse(p *parser, opt nodeOpt) {
	md.Operator = p.consumePrefixIn(modifierOps...)
	if md.Operator == "" {
		p.errorf("missing or invalid variable modifier, assuming ':-'")
		md.Operator = ":-"
	}
	md.Argument = parse(p, &Compound{}, opt&^inBackquotes)
}

// Lookahead.

func (p *parser) mayParseCommand(opt nodeOpt) bool {
	if opt&inBackquotes != 0 && p.nextIn("`") {
		return false
	}
	return p.nextInCompl(commandStopper)
}

func (p *parser) mayParseExpr(opt nodeOpt) bool {
	if opt&inBackquotes != 0 && p.nextIn("`") {
		return false
	}
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
