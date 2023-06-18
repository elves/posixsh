// Package parse implements parsing of POSIX shell scripts.
package parse

//go:generate stringer -type=RedirMode,PrimaryType,SegmentType -output=string.go

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
	parse(p, n, normal)
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

type nodeOpt uint

const (
	normal nodeOpt = iota
	// In backquotes, and not in a bracketed construct within.
	inBackquotes
	// Modifier arguments admit a larger range of characters as bareword
	// characters. See the comment before (*Primary).parse for details.
	modifierArg
)

// Chunk = sw { AndOr sw }
func (ch *Chunk) parse(p *parser, opt nodeOpt) {
	p.whitespaceOrSemicolon()
	for p.mayParseCommand(opt) {
		addTo(&ch.AndOrs, parse(p, &AndOr{}, opt))
		p.whitespaceOrSemicolon()
	}
}

// TODO: Only treat ) and } as command stoppers when there is an unclosed ( or
// {.
const commandStopper = "\r\n;)}&|"

func (p *parser) mayParseCommand(opt nodeOpt) bool {
	if opt == inBackquotes && p.nextIn("`") {
		return false
	}
	return p.nextInCompl(commandStopper)
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
	// One of SimpleCommand, FnDefCommand, GroupCommand,
	// SubshellGroupCommand, ForCommand, CaseCommand, IfCommand,
	// WhileCommand, UntilCommand
	Data    any
	Assigns []*Assign
	Redirs  []*Redir
}

type SimpleCommand struct {
	Words []*Compound
}

type FnDefCommand struct {
	Name *Compound
	Body *Form
}

type GroupCommand struct {
	Body *Chunk
}

type SubshellGroupCommand struct {
	Body *Chunk
}

var (
	assignPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z_0-9]*=`)
	redirPattern  = regexp.MustCompile(`^[0-9]*[<>]`)
)

func (fm *Form) parse(p *parser, opt nodeOpt) {
	p.whitespace()
	// Parse assignments, possibly mixed with redirections.
	for {
		if assignPattern.MatchString(p.rest()) {
			addTo(&fm.Assigns, parse(p, &Assign{}, opt))
			p.inlineWhitespace()
		} else if redirPattern.MatchString(p.rest()) {
			addTo(&fm.Redirs, parse(p, &Redir{}, opt))
			p.inlineWhitespace()
		} else {
			break
		}
	}
	switch {
	case p.maybeMeta("{"):
		fm.Data = GroupCommand{parse(p, &Chunk{}, normal)}
		p.meta("}")
	case p.maybeMeta("("):
		fm.Data = SubshellGroupCommand{parse(p, &Chunk{}, normal)}
		p.meta(")")
	case p.maybeWord("for", opt):
		p.inlineWhitespace()
		fm.Data = parseFor(p, opt)
	case p.maybeWord("case", opt):
		p.inlineWhitespace()
		fm.Data = parseCase(p, opt)
	case p.maybeWord("if", opt):
		p.inlineWhitespace()
		fm.Data = parseIf(p, opt)
	case p.maybeWord("while", opt):
		p.inlineWhitespace()
		fm.Data = parseWhile(p, opt)
	case p.maybeWord("until", opt):
		p.inlineWhitespace()
		fm.Data = parseUntil(p, opt)
	default:
		var words []*Compound
		for {
			if redirPattern.MatchString(p.rest()) {
				addTo(&fm.Redirs, parse(p, &Redir{}, opt))
			} else if p.mayParseExpr(opt) {
				addTo(&words, parse(p, &Compound{}, opt))
			} else {
				break
			}
			p.inlineWhitespace()
		}
		if len(words) == 1 && p.maybeMeta("(") {
			p.inlineWhitespace()
			p.meta(")")
			body := parse(p, &Form{}, opt)
			fm.Data = FnDefCommand{words[0], body}
		} else {
			fm.Data = SimpleCommand{words}
		}
	}
	for redirPattern.MatchString(p.rest()) {
		addTo(&fm.Redirs, parse(p, &Redir{}, opt))
	}
}

func (p *parser) maybeWord(s string, opt nodeOpt) bool {
	savePos := p.pos
	if p.consumePrefix(s) && !p.mayParseExpr(opt) {
		p.inlineWhitespace()
		return true
	}
	p.pos = savePos
	return false
}

type ForCommand struct {
	VarName *Compound
	// nil when there is no "in"; empty slice when "in" is followed by no word.
	Values []*Compound
	Body   []*AndOr
}

func parseFor(p *parser, opt nodeOpt) ForCommand {
	var fc ForCommand
	fc.VarName = parse(p, &Compound{}, opt)
	p.inlineWhitespace()
	if p.maybeWord("in", opt) {
		fc.Values = []*Compound{}
		p.inlineWhitespace()
		for p.mayParseExpr(opt) {
			addTo(&fc.Values, parse(p, &Compound{}, opt))
			p.inlineWhitespace()
		}
	}
	p.whitespaceOrSemicolon()
	fc.Body = parseDo(p, opt)
	return fc
}

type CaseCommand struct {
	Word     *Compound
	Patterns [][]*Compound
	Bodies   [][]*AndOr
}

func parseCase(p *parser, opt nodeOpt) CaseCommand {
	var cc CaseCommand
	cc.Word = parse(p, &Compound{}, opt)
	p.inlineWhitespace()
	if !p.maybeWord("in", opt) {
		p.errorf(`expect keyword "in"`)
	}
	p.whitespaceOrSemicolon()
	for {
		if p.maybeMeta("(") {
			p.inlineWhitespace()
		}
		var pattern []*Compound
		for {
			addTo(&pattern, parse(p, &Compound{}, opt))
			p.inlineWhitespace()
			if p.maybeMeta("|") {
				p.inlineWhitespace()
			} else {
				break
			}
		}
		if p.maybeMeta(")") {
			p.inlineWhitespace()
		} else {
			p.errorf(`expect ")"`)
		}
		seenDoubleSemicolon, seenEsac := false, false
		var body []*AndOr
		for p.mayParseCommand(opt) {
			if p.maybeWord("esac", opt) {
				p.whitespaceOrSemicolon()
				seenEsac = true
			}
			addTo(&body, parse(p, &AndOr{}, opt))
			p.whitespace()
			if p.maybeMeta(";;") {
				seenDoubleSemicolon = true
				break
			}
			p.whitespaceOrSemicolon()
		}
		addTo(&cc.Patterns, pattern)
		addTo(&cc.Bodies, body)
		if seenEsac {
			break
		}
		if !seenDoubleSemicolon {
			p.errorf(`expect ";;" or "esac"`)
			break
		}
	}
	return cc
}

type IfCommand struct {
	Conditions []*Chunk
	Bodies     []*Chunk
}

func parseIf(p *parser, opt nodeOpt) IfCommand {
	var ic IfCommand
	return ic
}

type WhileCommand struct {
	Condition *Chunk
	Body      *Chunk
}

func parseWhile(p *parser, opt nodeOpt) WhileCommand {
	var wc WhileCommand
	return wc
}

type UntilCommand struct {
	Condition *Chunk
	Body      *Chunk
}

func parseUntil(p *parser, opt nodeOpt) UntilCommand {
	var uc UntilCommand
	return uc
}

func parseDo(p *parser, opt nodeOpt) []*AndOr {
	var body []*AndOr
	if !p.maybeWord("do", opt) {
		p.errorf(`expect keyword "do"`)
	}
	p.whitespaceOrSemicolon()
	for p.mayParseCommand(opt) {
		if p.maybeWord("done", opt) {
			p.whitespaceOrSemicolon()
			return body
		}
		addTo(&body, parse(p, &AndOr{}, opt))
		p.whitespaceOrSemicolon()
	}
	p.errorf(`expect keyword "done"`)
	return body
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
	delim            string
	quoted           bool
	stripLeadingTabs bool
	Value            string
}

var leadingTabs = regexp.MustCompile(`(?m)^\t+`)

// This function is called in (*Whitespaces).parse immediately after a \n,
// for each pending Heredoc.
func (hd *Heredoc) parse(p *parser, _ struct{}) {
	begin := p.pos
	if hd.stripLeadingTabs {
		defer func() {
			hd.Value = leadingTabs.ReplaceAllLiteralString(hd.Value, "")
		}()
	}
	for i := p.pos; i < len(p.text); {
		j := i + strings.IndexByte(p.text[i:], '\n')
		iNext := j + 1
		if j == -1 {
			j = len(p.text)
			iNext = j
		}
		line := p.text[i:j]
		if hd.stripLeadingTabs {
			line = strings.TrimLeft(line, "\t")
		}
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

const digitSet = "0123456789"

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
	stripLeadingTabs := false
	switch {
	case p.maybeMeta(">>"):
		rd.Mode = RedirAppend
	case p.maybeMeta("<>"):
		rd.Mode = RedirInputOutput
	case p.maybeMeta("<<-"):
		rd.Mode = RedirHeredoc
		stripLeadingTabs = true
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
		rd.Heredoc = &Heredoc{delim: delim, quoted: quoted, stripLeadingTabs: stripLeadingTabs}
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

// Compound = [ TildePrefix ] { Primary }
type Compound struct {
	node
	TildePrefix string
	Parts       []*Primary
}

func (cp *Compound) parse(p *parser, opt nodeOpt) {
	// TODO: Parse TildePrefix correctly in assignment RHS
	if prefix := findTildePrefix(p.rest(), opt); prefix != "" {
		p.consume(len(prefix))
		cp.TildePrefix = prefix
	}
	for p.mayParseExpr(opt) {
		addTo(&cp.Parts, parse(p, &Primary{}, opt))
	}
}

const (
	// An expression is stopped where the command is stopped, another expression
	// starts (" \t), a redirection starts ("<>"), or at "(".
	//
	// "(" isn't really necessary as an expression stopper, but all of dash,
	// bash and ksh treat it as a syntax error in most places, and treating it
	// as an expression stopper simplifies the parsing of function definition
	// forms.
	normalExprStopper = commandStopper + " \t<>("

	inBackquotesExprStopper = normalExprStopper + "`"

	// A modifier argument (the "foo" in "${a:-foo}") is terminated by "}". The
	// usual expression stoppers are parsed as barewords characters; for
	// example, in "${a:-&|  foo}", the entire "&|  foo" part can be parsed as
	// one bareword. This is not specified by POSIX, but supported by all of
	// dash, bash and ksh.
	modifierArgExprStopper = " \t}"
)

var exprStopper = [...]string{
	normal:       normalExprStopper,
	inBackquotes: inBackquotesExprStopper,
	modifierArg:  modifierArgExprStopper,
}

func (p *parser) mayParseExpr(opt nodeOpt) bool {
	return p.nextInCompl(exprStopper[opt])
}

func findTildePrefix(s string, opt nodeOpt) string {
	if !hasPrefix(s, "~") {
		return ""
	}
	for i, r := range s {
		if i == 0 {
			// Skip the initial ~
			continue
		}
		if r == '/' || runeIn(r, exprStopper[opt]) {
			return s[:i]
		} else if runeIn(r, barewordStopper[opt]) {
			return ""
		}
	}
	return s
}

// Primary = Bareword | SingleQuoted | DoubleQuoted | WildcardChar | Arithmetic | OutputCapture | Variable
type Primary struct {
	node
	Type PrimaryType
	// String value. Valid for BarewordPrimary, SingleQuotedPrimary and
	// WildcardCharPrimary. For the first two types, the value contains the
	// processed value, e.g. the bareword \a has value "a".
	Value    string
	Variable *Variable  // Valid for VariablePrimary.
	Segments []*Segment // Valid for DoubleQuotesPrimary / ArithmeticPrimary.
	Body     *Chunk     // Valid for OutputCapturePrimary.
}

type PrimaryType int

const (
	InvalidPrimary PrimaryType = iota
	BarewordPrimary
	SingleQuotedPrimary
	DoubleQuotedPrimary
	ArithmeticPrimary
	WildcardCharPrimary
	OutputCapturePrimary
	VariablePrimary
)

const (
	nonBarewordStarter = "'\"$`[]?*"
	// A bareword primary stops where the entire expression stops, or another
	// non-bareword primary starts.
	normalBarewordStopper         = normalExprStopper + nonBarewordStarter
	normalVerbatimBarewordStopper = normalBarewordStopper + "\\"

	// See comment of modifierArgExprStopper.
	modifierArgBarewordStopper         = modifierArgExprStopper + nonBarewordStarter
	modifierArgVerbatimBarewordStopper = modifierArgBarewordStopper + "\\"
)

var barewordStopper = [...]string{
	normal:       normalBarewordStopper,
	inBackquotes: normalBarewordStopper,
	modifierArg:  modifierArgBarewordStopper,
}

var verbatimBarewordStopper = [...]string{
	normal:       normalVerbatimBarewordStopper,
	inBackquotes: normalVerbatimBarewordStopper,
	modifierArg:  modifierArgVerbatimBarewordStopper,
}

func (pr *Primary) parse(p *parser, opt nodeOpt) {
	barewordStopper := barewordStopper[opt]
	verbatimBarewordStopper := verbatimBarewordStopper[opt]

	switch {
	case p.nextInCompl(barewordStopper):
		pr.Type = BarewordPrimary
		// Optimization: Consume a prefix that does not contain backslashes.
		// This avoid building a strings.Builder when the bareword is free of
		// backslashes; we call that a "verbatim bareword".
		value := p.consumeWhileNotIn(verbatimBarewordStopper)
		if !p.hasPrefix("\\") {
			// One of barewordStopper runes or EOF was encounterd.
			pr.Value = value
			return
		}
		var sb strings.Builder
		sb.WriteString(value)
		lastBackslash := false
		p.consumeWhile(func(r rune) bool {
			if lastBackslash {
				sb.WriteRune(r)
				lastBackslash = false
				return true
			} else if r == '\\' {
				lastBackslash = true
				return true
			} else if runeIn(r, barewordStopper) {
				return false
			} else {
				sb.WriteRune(r)
				return true
			}
		})
		pr.Value = sb.String()
	case p.consumePrefix("'"):
		pr.Type = SingleQuotedPrimary
		begin := p.pos
		_ = p.consumeWhileNotIn("'")
		end := p.pos
		// recoverPos returns a position after all line continuations. When the
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
			addTo(&pr.Segments, parse(p, &Segment{}, (*int)(nil)))
		}
		if p.eof() {
			p.errorf("unterminated double-quoted string")
		}
	case p.consumePrefix("$(("):
		pr.Type = ArithmeticPrimary
		// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_06_04:
		// An arithmetic expression "shall be treated as if it were in
		// double-quotes, except that a double-quote inside the expression is
		// not treated specially. The shell shall expand all tokens in the
		// expression for parameter expansion, command substitution, and quote
		// removal".
		//
		// POSIX also doesn't specify when to parse "))" as the terminator of
		// the arithmetic expression and when to parse it as just two closing
		// parentheses, like in "$(( 1/(2*(1+2)) ))". What dash, bash and zsh
		// all do is to keep track of the parenthesis balance, and only parse
		// "))" as the terminator when there are no unmatched left parentheses.
		// We follow their behavior here, and stop parsing as soon as we see a
		// ")" with no matching "(".
		unmatchedLeftParens := 0
		for !p.eof() {
			if unmatchedLeftParens == 0 {
				if p.consumePrefix("))") {
					return
				} else if p.consumePrefix(")") {
					p.errorf(") in arithmetic expression with no matching (")
				}
			}
			addTo(&pr.Segments, parse(p, &Segment{}, &unmatchedLeftParens))
		}
		p.errorf("unterminated arithmetic expression")
	case p.hasPrefixIn("[", "]", "*", "?") != "":
		pr.Type = WildcardCharPrimary
		pr.Value = p.consume(1)
	case p.consumePrefix("`"):
		pr.Type = OutputCapturePrimary
		pr.Body = parse(p, &Chunk{}, inBackquotes)
		if !p.consumePrefix("`") {
			p.errorf("missing closing backquote for output capture")
		}
	case p.consumePrefix("$("):
		pr.Type = OutputCapturePrimary
		pr.Body = parse(p, &Chunk{}, normal)
		if !p.consumePrefix(")") {
			p.errorf("missing closing parenthesis for output capture")
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
		pr.parse(p, opt)
	}
}

type Segment struct {
	node
	Type      SegmentType
	Value     string
	Expansion *Primary
}

type SegmentType int

const (
	InvalidSegment SegmentType = iota
	StringSegment
	ExpansionSegment
)

var (
	dqStringSegmentStopper        = "$`\""
	dqLiteralStringSegmentStopper = dqStringSegmentStopper + "\\"
)

// Parses a segment inside "" (if unmatchedLeftParens == nil) or $(( )).
func (seg *Segment) parse(p *parser, unmatchedLeftParens *int) {
	switch {
	case p.hasPrefixIn("$", "`") != "":
		seg.Type = ExpansionSegment
		seg.Expansion = parse(p, &Primary{}, normal)
	case unmatchedLeftParens == nil:
		seg.Type = StringSegment
		// Optimization: Consume a prefix that does not contain backslashes.
		// This avoids building a bytes.Buffer when this segment is free of
		// backslashes.
		raw := p.consumeWhileNotIn(dqLiteralStringSegmentStopper)
		if !p.hasPrefix("\\") {
			seg.Value = raw
			return
		}
		var b strings.Builder
		b.WriteString(raw)
		lastBackslash := false
		p.consumeWhile(func(r rune) bool {
			if lastBackslash {
				if !runeIn(r, dqLiteralStringSegmentStopper) {
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
		seg.Value = b.String()
	default:
		seg.Type = StringSegment
		// POSIX says that an arithmetic expression "shall be treated as if it
		// were in double-quotes", meaning that \ should be able to escape $ and
		// `. However, since a literal $ or ` is invalid inside arithmetic
		// expressions anyway, we don't actually need to handle this.
		seg.Value = p.consumeWhile(func(r rune) bool {
			switch r {
			case '(':
				*unmatchedLeftParens++
				return true
			case ')':
				// Stop parsing as soon we see a ")" with no matching "(". See
				// the comment in Primary.parse for more context.
				if *unmatchedLeftParens == 0 {
					return false
				}
				*unmatchedLeftParens--
				return true
			case '$', '`':
				return false
			default:
				return true
			}
		})
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
		if p.hasPrefixIn("-}", "?}") != "" {
			// ${#-} and ${#?} are ambiguous, and can be parsed either as the
			// length of $- and $?, or as an operator following $# with an empty
			// argument.
			//
			// POSIX doesn't say definitely which behavior is correct, but seems
			// to prefer the former interpretation by saying that application
			// should not use an empty argument if the latter is desired. In
			// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_06_02:
			// > If parameter is '#' and the colon is omitted, the application
			// > shall ensure that word is specified (this is necessary to avoid
			// > ambiguity with the string length expansion)
			//
			// This behavior is consistent with dash, bash and zsh.
			va.LengthOp = true
			va.Name = p.consume(1)
			// va.Name = parseVariableName(p, true)
		} else if p.hasPrefixIn("=}", "+}") != "" {
			// Similarly to the previous case, ${#=} and ${#+} are also
			// ambiguous, but POSIX seems to prefer to parse them as the length
			// of $= and $+. However, since these variables are invalid, this
			// will result immediately in a parse error.
			//
			// The alternative interpretation of an operator following $# is
			// technically possible but not useful, since $# is always set and
			// non-null.
			//
			// This behavior is consistent with dash and bash, but zsh uses the
			// alternative interpretation.
			p.errorf("invalid parameter substitution ${#%s}, treating as ${#}", p.consume(1))
			va.Name = "#"
		} else if p.hasPrefix("}") || p.hasPrefixIn(modifierOps...) != "" {
			va.Name = "#"
		} else {
			va.LengthOp = true
			va.Name = parseVariableName(p, true)
			// POSIX doesn't specify whether the prefix # operator can coexist
			// with the suffix operators; we permit it. The Evaler evaluates the
			// suffix operator before applying the length operator.
			//
			// This behavior is consistent with zsh; dash and bash both forbid
			// it.
		}
	} else {
		va.Name = parseVariableName(p, true)
	}
	if p.hasPrefixNot("}") {
		va.Modifier = parse(p, &Modifier{}, opt)
		if va.LengthOp {
			p.errorf("variable expansion has both length operator and modifier")
		}
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
	// TODO: Most other shells (dash, bash and ksh) allow the modifier argument
	// to contain multiple words, even though it's not specified in POSIX.
	// Support that too.
	md.Argument = parse(p, &Compound{}, modifierArg)
}

// Lookahead.

func (p *parser) nextIn(set string) bool {
	r, size := utf8.DecodeRuneInString(p.rest())
	return size > 0 && runeIn(r, set)
}

func (p *parser) nextInCompl(set string) bool {
	r, size := utf8.DecodeRuneInString(p.rest())
	return size > 0 && !runeIn(r, set)
}
