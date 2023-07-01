// Package parse implements parsing of POSIX shell scripts.
package parse

//go:generate stringer -type=RedirMode,PrimaryType -output=string.go

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
	Not      bool
	Commands []*Command
}

// Pipeline = Form iw { ("|" \ "||") w Form iw }
func (pl *Pipeline) parse(p *parser, opt nodeOpt) {
	pl.Not = p.maybeWord("!", opt)
	addTo(&pl.Commands, parse(p, &Command{}, opt))
	p.inlineWhitespace()
	for p.hasPrefix("|") && !p.hasPrefix("||") {
		// | should be meta
		p.consumePrefix("|")
		p.whitespace()
		addTo(&pl.Commands, parse(p, &Command{}, opt))
		p.inlineWhitespace()
	}
}

type Command struct {
	node
	// One of SimpleCommand, FnDefCommand, GroupCommand,
	// SubshellGroupCommand, ForCommand, CaseCommand, IfCommand,
	// WhileCommand, UntilCommand
	Data    any
	Assigns []*Assign
	Redirs  []*Redir
}

type Simple struct {
	Words []*Compound
}

type FnDef struct {
	Name *Compound
	Body *Command
}

type Group struct {
	Body *Chunk
}

type SubshellGroup struct {
	Body *Chunk
}

var (
	assignPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z_0-9]*=`)
	redirPattern  = regexp.MustCompile(`^[0-9]*[<>]`)
)

func (fm *Command) parse(p *parser, opt nodeOpt) {
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
		fm.Data = Group{parse(p, &Chunk{}, normal)}
		p.meta("}")
		p.inlineWhitespace()
	case p.maybeMeta("("):
		fm.Data = SubshellGroup{parse(p, &Chunk{}, normal)}
		p.meta(")")
		p.inlineWhitespace()
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
			body := parse(p, &Command{}, opt)
			fm.Data = FnDef{words[0], body}
		} else {
			fm.Data = Simple{words}
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

type For struct {
	VarName *Compound
	// nil when there is no "in"; empty slice when "in" is followed by no word.
	Values []*Compound
	Body   []*AndOr
}

func parseFor(p *parser, opt nodeOpt) For {
	var data For
	data.VarName = parse(p, &Compound{}, opt)
	p.inlineWhitespace()
	if p.maybeWord("in", opt) {
		data.Values = []*Compound{}
		p.inlineWhitespace()
		for p.mayParseExpr(opt) {
			addTo(&data.Values, parse(p, &Compound{}, opt))
			p.inlineWhitespace()
		}
	}
	p.whitespaceOrSemicolon()
	if !p.maybeWord("do", opt) {
		p.errorf(`expect keyword "do"`)
	}
	p.whitespaceOrSemicolon()
	data.Body = parseAndOrsTerminatedBy(p, opt, "done")
	return data
}

type Case struct {
	Word     *Compound
	Patterns [][]*Compound
	Bodies   [][]*AndOr
}

func parseCase(p *parser, opt nodeOpt) Case {
	var data Case
	data.Word = parse(p, &Compound{}, opt)
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
		addTo(&data.Patterns, pattern)
		addTo(&data.Bodies, body)
		if seenEsac {
			break
		}
		if !seenDoubleSemicolon {
			p.errorf(`expect ";;" or "esac"`)
			break
		}
	}
	return data
}

type If struct {
	Conditions [][]*AndOr
	Bodies     [][]*AndOr
	ElseBody   []*AndOr
}

func parseIf(p *parser, opt nodeOpt) If {
	var data If
branches:
	for {
		addTo(&data.Conditions, parseAndOrsTerminatedBy(p, opt, "then"))
		var body []*AndOr
		for p.mayParseCommand(opt) {
			if p.maybeWord("fi", opt) {
				p.whitespaceOrSemicolon()
				addTo(&data.Bodies, body)
				return data
			} else if p.maybeWord("else", opt) {
				p.whitespaceOrSemicolon()
				addTo(&data.Bodies, body)
				data.ElseBody = parseAndOrsTerminatedBy(p, opt, "fi")
				return data
			} else if p.maybeWord("elif", opt) {
				p.whitespaceOrSemicolon()
				addTo(&data.Bodies, body)
				continue branches
			}
			addTo(&body, parse(p, &AndOr{}, opt))
			p.whitespaceOrSemicolon()
		}
		p.whitespaceOrSemicolon()
		addTo(&data.Bodies, body)
		p.errorf(`expect "fi", "else" or "elif"`)
		return data
	}
}

type While struct {
	Condition []*AndOr
	Body      []*AndOr
}

func parseWhile(p *parser, opt nodeOpt) While {
	condition, body := parseWhileUntil(p, opt)
	return While{condition, body}
}

type Until struct {
	Condition []*AndOr
	Body      []*AndOr
}

func parseUntil(p *parser, opt nodeOpt) Until {
	condition, body := parseWhileUntil(p, opt)
	return Until{condition, body}
}

func parseWhileUntil(p *parser, opt nodeOpt) (condition, body []*AndOr) {
	condition = parseAndOrsTerminatedBy(p, opt, "do")
	body = parseAndOrsTerminatedBy(p, opt, "done")
	return
}

func parseAndOrsTerminatedBy(p *parser, opt nodeOpt, word string) []*AndOr {
	var body []*AndOr
	for p.mayParseCommand(opt) {
		if p.maybeWord(word, opt) {
			p.whitespaceOrSemicolon()
			return body
		}
		addTo(&body, parse(p, &AndOr{}, opt))
		p.whitespaceOrSemicolon()
	}
	p.errorf(`expect keyword "%s"`, word)
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
	// If non-nil, the heredoc is unquoted and the parsed segments is stored
	// inside this slice. Otherwise the heredoc is quoted and the literal text
	// (after possibly stripping leading tabs) is in the Text field.
	Segments []Segment
	Text     string
}

var leadingTabs = regexp.MustCompile(`(?m)^\t+`)

// This function is called in (*Whitespaces).parse immediately after a \n,
// for each pending Heredoc.
func (hd *Heredoc) parse(p *parser, ph *pendingHeredoc) {
	var tabPrefix string
	if ph.stripLeadingTabs {
		tabPrefix = `\t*`
	}
	endRegexp := regexp.MustCompile(`(?m)^` + tabPrefix + regexp.QuoteMeta(ph.delim) + `$\n?`)
	endLoc := endRegexp.FindStringIndex(p.rest())
	if endLoc == nil {
		p.errorf("undelimited heredoc %q", ph.delim)
	}

	if ph.quoted {
		if endLoc == nil {
			hd.Text = p.text[p.pos:]
			p.pos = len(p.text)
		} else {
			hd.Text = p.text[p.pos : p.pos+endLoc[0]]
			p.pos += endLoc[1]
		}
		if ph.stripLeadingTabs {
			hd.Text = leadingTabs.ReplaceAllLiteralString(hd.Text, "")
		}
	} else {
		// TODO: Support stripLeadingTabs
		begin := p.pos
		savedText := p.text
		if endLoc != nil {
			// Hack: clip the text when parsing segments so that parsing of
			// expansion segments doesn't consume the heredoc delimiter.
			p.text = p.text[:p.pos+endLoc[0]]
		}
		for !p.eof() {
			addTo(&hd.Segments, Segment(parse(p, &HeredocSegment{}, ph.stripLeadingTabs)))
		}
		p.text = savedText
		if endLoc == nil {
			p.pos = len(p.text)
		} else {
			p.pos = begin + endLoc[1]
		}
	}
}

type HeredocSegment struct {
	node
	Expansion *Primary
	Text      string
}

func (seg *HeredocSegment) Segment() (*Primary, string) { return seg.Expansion, seg.Text }

var newlineAndTabs = regexp.MustCompile(`\n\t+`)

func (seg *HeredocSegment) parse(p *parser, stripLeadingTabs bool) {
	if p.hasPrefixIn("$", "`") != "" {
		// TODO: Strip leading tabs when parsing expansions too. Leading tabs
		// are meaningful when inside string literals or just after a line
		// continuation.
		seg.Expansion = parse(p, &Primary{}, normal)
	} else {
		sol := p.pos == 0 || p.text[p.pos-1] == '\n'
		seg.Text = parseStringSegment(p, "$`")
		if stripLeadingTabs {
			seg.Text = newlineAndTabs.ReplaceAllLiteralString(seg.Text, "\n")
			if sol {
				seg.Text = strings.TrimLeft(seg.Text, "\t")
			}
		}
	}
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
	RedirOutputOverwrite
	RedirInputOutput
	RedirAppend
	RedirHeredoc
)

const digitSet = "0123456789"

// Redir = `[0-9]*` (">>" | "<>" | "<<-" | "<<" | ">|" | ">" | "<") w [ "&" w ] Compound
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
	case p.maybeMeta(">|"):
		rd.Mode = RedirOutputOverwrite
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
		rd.Heredoc = &Heredoc{}
		pending := &pendingHeredoc{delim, quoted, stripLeadingTabs, rd.Heredoc}
		p.pendingHeredocs = append(p.pendingHeredocs, pending)
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
	//
	// TODO: Also admit spaces and tabs.
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
	// String value. Valid for BarewordPrimary, EscapedPrimary and
	// SingleQuotedPrimary. For the latter two types, the value contains the
	// string content without the delimiters.
	Value    string
	Variable *Variable // Valid for VariablePrimary.
	Segments []Segment // Valid for DoubleQuotesPrimary / ArithmeticPrimary.
	Body     *Chunk    // Valid for OutputCapturePrimary.
}

type Segment interface {
	Segment() (*Primary, string)
}

type PrimaryType int

const (
	InvalidPrimary PrimaryType = iota
	// Barewords includes glob characters [ ] ? *, but not escaped characters
	// like \*.
	BarewordPrimary
	EscapedPrimary
	SingleQuotedPrimary
	DoubleQuotedPrimary
	ArithmeticPrimary
	OutputCapturePrimary
	VariablePrimary
)

const (
	nonBarewordStarter = "\\'\"$`"
	// A bareword primary stops where the entire expression stops, or another
	// non-bareword primary starts.
	normalBarewordStopper = normalExprStopper + nonBarewordStarter
	// See comment of modifierArgExprStopper.
	modifierArgBarewordStopper = modifierArgExprStopper + nonBarewordStarter
)

var barewordStopper = [...]string{
	normal:       normalBarewordStopper,
	inBackquotes: normalBarewordStopper,
	modifierArg:  modifierArgBarewordStopper,
}

func (pr *Primary) parse(p *parser, opt nodeOpt) {
	switch {
	case p.nextInCompl(barewordStopper[opt]):
		pr.Type = BarewordPrimary
		pr.Value = p.consumeWhileNotIn(barewordStopper[opt])
	case p.consumePrefix("\\"):
		pr.Type = EscapedPrimary
		// Note: Line continuations are already handled.
		if p.eof() {
			// This behavior doesn't seem to be specified by POSIX, but all of
			// dash, bash, ksh (but not zsh) treat \ before EOF as a literal \.
			pr.Value = "\\"
		} else {
			_, len := utf8.DecodeRuneInString(p.rest())
			pr.Value = p.consume(len)
		}
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
		for !p.eof() && !p.hasPrefix(`"`) {
			addTo(&pr.Segments, Segment(parseNoOpt(p, &DQSegment{})))
		}
		if p.eof() {
			p.errorf("unterminated double-quoted string")
		}
		p.consumePrefix(`"`)
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
			addTo(&pr.Segments, Segment(parse(p, &ArithSegment{}, &unmatchedLeftParens)))
		}
		p.errorf("unterminated arithmetic expression")
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

// DQSegment represents a segment inside double-quoted strings, either an
// expansion (in which case Expansion is non-nil) or literal text (in which case
// Text is non-empty).
type DQSegment struct {
	node
	Expansion *Primary
	Text      string
}

func (seg *DQSegment) Segment() (*Primary, string) { return seg.Expansion, seg.Text }

// Parses a segment inside "".
func (seg *DQSegment) parse(p *parser, _ struct{}) {
	if p.hasPrefixIn("$", "`") != "" {
		seg.Expansion = parse(p, &Primary{}, normal)
	} else {
		seg.Text = parseStringSegment(p, "$`\"")
	}
}

func parseStringSegment(p *parser, meta string) string {
	// Optimization: Consume a prefix that does not contain backslashes.
	// This avoids building a strings.Builder when this segment is free of
	// backslashes.
	raw := p.consumeWhileNotIn(meta + `\`)
	if !p.hasPrefix(`\`) {
		return raw
	}
	var b strings.Builder
	b.WriteString(raw)
	lastBackslash := false
	p.consumeWhile(func(r rune) bool {
		if lastBackslash {
			if !runeIn(r, meta+`\`) {
				b.WriteRune('\\')
			}
			b.WriteRune(r)
			lastBackslash = false
			return true
		} else if r == '\\' {
			lastBackslash = true
			return true
		} else if runeIn(r, meta) {
			return false
		} else {
			b.WriteRune(r)
			return true
		}
	})
	return b.String()
}

// ArithSegment represents a segment in an arithmetic expression, either an
// expansion (in which case Expansion is non-nil) or literal text (in which case
// Text is non-empty).
type ArithSegment struct {
	node
	Expansion *Primary
	Text      string
}

func (seg *ArithSegment) Segment() (*Primary, string) { return seg.Expansion, seg.Text }

func (seg *ArithSegment) parse(p *parser, unmatchedLeftParens *int) {
	if p.hasPrefixIn("$", "`") != "" {
		seg.Expansion = parse(p, &Primary{}, normal)
		return
	}
	// POSIX says that an arithmetic expression "shall be treated as if it
	// were in double-quotes", meaning that \ should be able to escape $ and
	// `. However, since a literal $ or ` is invalid inside arithmetic
	// expressions anyway, we don't actually need to handle this.
	seg.Text = p.consumeWhile(func(r rune) bool {
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
