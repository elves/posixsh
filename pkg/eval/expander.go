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
// methods for performing the further expansions. Note that since pathname
// expansion happens after compounding, an expander doesn't actually perform
// pathname expansion, but expands into glob patterns (represented by globWord).
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
	// Perform field splitting and pre-globbing, if applicable.
	expand(ifs string) []globWord
	// Expand without field splitting or pathname expansion. The result must be
	// one word.
	expandOneWord() string
}

type globWord []globWordSegment

// A union of either one globbing metacharacter (one of ? * [ ]) or a segment of
// literal text.
type globWordSegment struct {
	// If 0, this segment is a run of literal text.
	meta byte
	text string
}

// An scalar scalar that is subject to field splitting and pre-globbing.
type scalar struct{ s string }

func (u scalar) expand(ifs string) []globWord { return parseGlob(split(u.s, ifs)) }
func (u scalar) expandOneWord() string        { return u.s }

// A literal that is *not* subject to field splitting and pathname expansion.
type literal struct{ s string }

func (l literal) expand(ifs string) []globWord { return []globWord{{{text: l.s}}} }
func (l literal) expandOneWord() string        { return l.s }

// A globbing metacharacter.
type globMeta struct{ m byte }

func (gm globMeta) expand(ifs string) []globWord { return []globWord{{{meta: gm.m}}} }
func (gm globMeta) expandOneWord() string        { return string([]byte{gm.m}) }

// Evaluation result of a compound expression.
type compound struct{ elems []expander }

func (c compound) expand(ifs string) []globWord {
	return expandElems(c.elems, func(e expander) []globWord {
		return e.expand(ifs)
	})
}

func (c compound) expandOneWord() string {
	return expandOneWordElems(c.elems)
}

// Evaluation result of a double-quoted string.
type doubleQuoted struct{ elems []expander }

func (dq doubleQuoted) expand(ifs string) []globWord {
	return expandElems(dq.elems, func(e expander) []globWord {
		// Special-case $@ inside double quotes.
		if a, ok := e.(array); ok && a.isAt {
			words := make([]globWord, len(a.elems))
			for i, elem := range a.elems {
				words[i] = globWord{{text: elem}}
			}
			return words
		}
		return []globWord{{{text: e.expandOneWord()}}}
	})
}

func (dq doubleQuoted) expandOneWord() string {
	return expandOneWordElems(dq.elems)
}

// $* or $@, or the result of applying a trimming operator to them. Both have
// complex word splitting behavior, described in
// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_05_02.
// The behavior of $@ inside double quotes is implemented in
// doubleQuoted.expand.
type array struct {
	elems []string
	ifs   func() string // Needed for expandOneWord
	isAt  bool
}

func (a array) expand(ifs string) []globWord {
	var words []globWord
	for _, arg := range a.elems {
		if arg != "" {
			words = append(words, parseGlob(split(arg, ifs))...)
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

func expandElems(elems []expander, f func(expander) []globWord) []globWord {
	var words []globWord
	for _, elem := range elems {
		more := f(elem)
		if len(words) == 0 {
			words = more
		} else if len(more) > 0 {
			words[len(words)-1] = appendGlobWord(words[len(words)-1], more[0])
			words = append(words, more[1:]...)
		}
	}
	return words
}

func appendGlobWord(w1, w2 globWord) globWord {
	if len(w1) > 0 && len(w2) > 0 && w1[len(w1)-1].text != "" && w2[0].text != "" {
		w1[len(w1)-1].text += w2[0].text
		w2 = w2[1:]
	}
	return append(w1, w2...)
}

func expandOneWordElems(elems []expander) string {
	var sb strings.Builder
	for _, elem := range elems {
		sb.WriteString(elem.expandOneWord())
	}
	return sb.String()
}

func parseGlob(fields []string) []globWord {
	ts := make([]globWord, len(fields))
	for i, s := range fields {
		ts[i] = parseGlobOne(s)
	}
	return ts
}

func parseGlobOne(s string) globWord {
	var segs []globWordSegment
	for s != "" {
		i := strings.IndexAny(s, "[]?*")
		if i == -1 {
			segs = append(segs, globWordSegment{text: s})
			break
		}
		if i > 0 {
			segs = append(segs, globWordSegment{text: s[:i]})
		}
		segs = append(segs, globWordSegment{meta: s[i]})
		s = s[i+1:]
	}
	return segs
}

func split(s, ifs string) []string {
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
