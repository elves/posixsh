package eval

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// POSIX groups expansions into three steps:
//
//  1. Tilde expansion, parameter expansion, command substitution and arithmetic
//     expansion.
//  2. Field spltting.
//  3. Pathname expansion.
//
// (POSIX also specifies a "quote removal" step, but it's an artifact of the
// evaluation model assumed by POSIX, and doesn't apply to this implementation.)
//
// Expansions in step 1 can be parsed statically and done together. Step 2 and 3
// are done dynamically on the result of step 1, apply depends on the
// syntactical environment. (Pathname expansion can be turned off globally by
// "set -f". This discussion is about whether it *would* apply if "set -f" were
// not in effect.)
//
// For example, in "echo $x", $x is subject to field splitting and path
// expansion, whereas in "y=$x", $x is not. They always go together; there is no
// syntactical environment in POSIX that only applies field splitting or only
// applies pathname expansion.
//
// In this implementation, step 1 results in an "expander", which provides
// methods for performing the further expansions.
//
// A simpler alternative to this approach is already deciding whether field
// splitting and pathname expansion should be done when evaluating the
// expression. This works for all cases except one: in "echo ${y:=$x"*"}", if $y
// is unset or null, $x"*" is expanded in both manners:
//
//  1. The result without field splitting or pathname expansion is used to
//     assign to $y.
//  2. The result with those two expansions is used as command arguments.
//
// In this implementation, this case can't be modeled as taking the string
// result of step 1 and applying field splitting and pathname expansion later on
// to get the result of step 2, because the string result doesn't preserve
// quotes and would cause the quoted "*" to be expanded incorrectly. (An
// implementation that follows POSIX's evaluation model and preserves quotes
// during the intermediate stages can work like this, but that's not how this
// implementation works.)
//
// TODO: Actually implement pathname expansion.
type expander interface {
	// Expand with field splitting and pathname expansion.
	expand(ifs string) []string
	// Expand without field splitting or pathname expansion. The result must be
	// one word.
	expandOneWord() string
}

// An scalar scalar that is subject to field splitting and pathname expansion.
type scalar struct{ s string }

func (u scalar) expand(ifs string) []string { return splitWords(u.s, ifs) }
func (u scalar) expandOneWord() string      { return u.s }

// A literal that is *not* subject to field splitting and pathname expansion.
type literal struct{ s string }

func (l literal) expand(ifs string) []string { return []string{l.s} }
func (l literal) expandOneWord() string      { return l.s }

// Evaluation result of a compound expression.
type compound struct{ elems []expander }

func (c compound) expand(ifs string) []string {
	cc := concatter{}
	for _, elem := range c.elems {
		cc.concat(elem.expand(ifs))
	}
	return cc.words
}

func (c compound) expandOneWord() string {
	var sb strings.Builder
	for _, elem := range c.elems {
		sb.WriteString(elem.expandOneWord())
	}
	return sb.String()
}

// Evaluation result of a double-quoted string.
type doubleQuoted struct{ elems []expander }

func (dq doubleQuoted) expand(ifs string) []string {
	cc := concatter{}
	for _, elem := range dq.elems {
		cc.concat(expandInDoubleQuotes(elem))
	}
	return cc.words
}

func (dq doubleQuoted) expandOneWord() string {
	var sb strings.Builder
	for _, elem := range dq.elems {
		sb.WriteString(elem.expandOneWord())
	}
	return sb.String()
}

// An optional interface that can be implemented by an expander.
type doubleQuotesExpander interface {
	// Expand in double quotes. This is the same as expandOneWord for all
	// expressions except $@, which results in multiple words.
	expandInDoubleQuotes() []string
}

func expandInDoubleQuotes(e expander) []string {
	if dqe, ok := e.(doubleQuotesExpander); ok {
		return dqe.expandInDoubleQuotes()
	}
	return []string{e.expandOneWord()}
}

// $* or $@, or the result of applying a trimming operator to them. Both have
// complex word splitting behavior, described in
// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_05_02.
type array struct {
	elems []string
	ifs   func() string
	isAt  bool
}

func (a array) expand(ifs string) []string {
	var words []string
	for _, arg := range a.elems {
		if arg != "" {
			words = append(words, splitWords(arg, ifs)...)
		}
	}
	return words
}

func (a array) expandOneWord() string {
	// POSIX leaves unspecified how $@ expands in a one-word environment; we let
	// it behave like $*.
	var sep string
	if ifs := a.ifs(); ifs != "" {
		r, _ := utf8.DecodeRuneInString(ifs)
		sep = string(r)
	}
	return strings.Join(a.elems, sep)
}

func (a array) expandInDoubleQuotes() []string {
	if a.isAt {
		return a.elems
	} else {
		return []string{a.expandOneWord()}
	}
}

func splitWords(s, ifs string) []string {
	// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_06_05
	//
	// The following implements the algorithm described in clause 3. Clause 1
	// describes the default behavior, but it's consistent with the more general
	// clause 3.
	//
	// The algorithm depends on a definition of "character", which is not
	// explicitly specified in this section. This detail is important when IFS
	// contains multi-byte codepoints. Dash seems to treat each byte as a
	// character, whereas both ksh and bash treats each codepoint as a
	// character. We follow the behavior of ksh and bash because it makes more
	// sense.
	if ifs == "" {
		return []string{s}
	}
	// The following implements the algorithm described in clause 3. Clause 1
	// describes the default behavior, but it's consistent with the more general
	// clause 3.
	//
	// The algorithm depends on a definition of "character", which is not
	// explicitly specified in this section. This detail is important when IFS
	// contains multi-byte codepoints. Dash seems to treat each byte as a
	// character, whereas both ksh and bash treats each codepoint as a
	// character. We follow the behavior of ksh and bash because it makes more
	// sense.
	var whitespaceRunes, nonWhitespaceRunes []rune
	for _, r := range ifs {
		if r == ' ' || r == '\t' || r == '\n' {
			whitespaceRunes = append(whitespaceRunes, r)
		} else {
			nonWhitespaceRunes = append(nonWhitespaceRunes, r)
		}
	}
	whitespaces := string(whitespaceRunes)
	nonWhitespaces := string(nonWhitespaceRunes)

	// a. Ignore leading and trailing IFS whitespaces.
	s = strings.Trim(s, whitespaces)

	delimPatterns := make([]string, 0, 2)
	// b. Each occurrence of a non-whitespace IFS character, with optional
	// leading and trailing IFS whitespaces, are considered delimiters.
	if nonWhitespaces != "" {
		p := "[" + regexp.QuoteMeta(nonWhitespaces) + "]"
		if whitespaces != "" {
			whitePattern := "[" + regexp.QuoteMeta(whitespaces) + "]*"
			p = whitePattern + p + whitePattern
		}
		delimPatterns = append(delimPatterns, p)
	}
	// c. Non-zero-length IFS white space shall delimit a field.
	if whitespaces != "" {
		p := "[" + regexp.QuoteMeta(whitespaces) + "]+"
		delimPatterns = append(delimPatterns, p)
	}

	// Apply splitting from rule b and c.
	//
	// TODO: Cache the compiled regexp.
	return regexp.MustCompile(strings.Join(delimPatterns, "|")).Split(s, -1)
}

type concatter struct{ words []string }

func (c *concatter) concat(more []string) {
	if len(c.words) == 0 {
		c.words = more
	} else if len(more) > 0 {
		c.words[len(c.words)-1] += more[0]
		c.words = append(c.words, more[1:]...)
	}
}
