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
// Expansions in step 1 can be parsed statically and done in the same pass. Step
// 2 and 3 are done dynamically on the result of step 1, and whether they apply
// depends on the syntactical environment.
//
// (Note: Pathname expansion can be turned off globally by "set -f". This
// discussion is about whether it *would* apply if "set -f" were not in effect.)
//
// For example, in "echo $x", $x is subject to field splitting and path
// expansion, whereas in "y=$x", $x is not subject to either. There are also two
// environments that doesn't perform either but recognizes globbing characters
// for pattern matching instead of pathname expansion: the "foo*" in both
// "${x%%foo*}" and in "case foo in; foo*) ... esac".
//
// In this implementation, the intermediate result from step 1 is represented by
// an "expander", which provides methods for performing the further expansions.
// Since pathname expansion happens after compound words are joined, an expander
// never performs actual pathname expansion, but can produce an intermediate
// data structure, "word", that indicates which parts are unquoted for the
// purpose of parsing wildcard characters.
//
// A simpler alternative to this approach is deciding whether field splitting
// and parsing of wildcard characters should be done when evaluating the
// expression. This works for all cases except one: in "echo ${y:=$x"*"}", if $y
// is unset or null, $x"*" is expanded in two ways:
//
//  1. The result without field splitting or parsing of wildcard characters is
//     used to assign to $y.
//  2. The result with those two expansions is used as command arguments.
//
// This case can't be easily modeled as taking the words resulting from step 1
// and applying field splitting and parsing of wildcard characters to get the
// result of step 2: the rules for $* and $@ are different depending on whether
// the environment requires field splitting.
type expander interface {
	// Expand with field splitting and parsing of globbing characters.
	expand(ifs string) []word
	// Expand without field splitting, but with parsing of glob characters. This
	// always results in one word.
	expandOneWord() word
	// Expand without field splitting or parsing of glob characters. This always
	// results in one string.
	expandOneString() string
}

type word []wordSegment

// A union of either one wildcard character (one of ? * [ ]) or a segment of
// literal text.
type wordSegment struct {
	// If 0, this segment is a run of literal text.
	meta byte
	text string
}

// A scalar that is subject to field splitting and parsing of glob characters.
type scalar struct{ s string }

func (u scalar) expand(ifs string) []word { return parseGlob(split(u.s, ifs)) }
func (u scalar) expandOneWord() word      { return parseGlobOne(u.s) }
func (u scalar) expandOneString() string  { return u.s }

// A literal that is *not* subject to field splitting and pathname expansion.
type literal struct{ s string }

func (l literal) expand(ifs string) []word { return []word{l.expandOneWord()} }
func (l literal) expandOneWord() word      { return word{{text: l.s}} }
func (l literal) expandOneString() string  { return l.s }

// A globbing metacharacter.
type globMeta struct{ m byte }

func (gm globMeta) expand(ifs string) []word { return []word{gm.expandOneWord()} }
func (gm globMeta) expandOneWord() word      { return word{{meta: gm.m}} }
func (gm globMeta) expandOneString() string  { return string([]byte{gm.m}) }

// Evaluation result of a compound expression.
type compound struct{ elems []expander }

func (c compound) expand(ifs string) []word {
	return expandFromElems(nil, c.elems, func(e expander) []word {
		return e.expand(ifs)
	})
}

func (c compound) expandOneWord() word     { return expandOneWordFromElems(c.elems) }
func (c compound) expandOneString() string { return expandOneStringFromElems(c.elems) }

// Evaluation result of a double-quoted string.
type doubleQuoted struct{ elems []expander }

func (dq doubleQuoted) expand(ifs string) []word {
	return expandFromElems([]word{nil}, dq.elems, func(e expander) []word {
		// Special-case $@ inside double quotes.
		if a, ok := e.(array); ok && a.isAt {
			words := make([]word, len(a.elems))
			for i, elem := range a.elems {
				words[i] = word{{text: elem}}
			}
			return words
		}
		return []word{{{text: e.expandOneString()}}}
	})
}

func (dq doubleQuoted) expandOneWord() word     { return expandOneWordFromElems(dq.elems) }
func (dq doubleQuoted) expandOneString() string { return expandOneStringFromElems(dq.elems) }

// $* or $@, or the result of applying a trimming operator to them. Both have
// complex word splitting behavior, described in
// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_05_02.
// The behavior of $@ inside double quotes is implemented in
// doubleQuoted.expand.
type array struct {
	elems []string
	ifs   func() string // Needed for expandOneString
	isAt  bool
}

func (a array) expand(ifs string) []word {
	var words []word
	for _, arg := range a.elems {
		if arg != "" {
			words = append(words, parseGlob(split(arg, ifs))...)
		}
	}
	return words
}

func (a array) expandOneWord() word { return parseGlobOne(a.expandOneString()) }

func (a array) expandOneString() string {
	// POSIX leaves unspecified how $@ expands in a one-word environment; we let
	// it behave like $*.
	var sep string
	if ifs := a.ifs(); ifs != "" {
		r, _ := utf8.DecodeRuneInString(ifs)
		sep = string(r)
	}
	return strings.Join(a.elems, sep)
}

// Provides expansion by concatenating the expansion of elems, using initWords
// as the initial value for the expansion result, and the f function to expand
// each element.
func expandFromElems(initWords []word, elems []expander, f func(expander) []word) []word {
	words := initWords
	for _, elem := range elems {
		more := f(elem)
		if len(words) == 0 {
			words = more
		} else if len(more) > 0 {
			words[len(words)-1] = appendWord(words[len(words)-1], more[0])
			words = append(words, more[1:]...)
		}
	}
	return words
}

func expandOneWordFromElems(elems []expander) word {
	var w word
	for _, elem := range elems {
		w = appendWord(w, elem.expandOneWord())
	}
	return w
}

func expandOneStringFromElems(elems []expander) string {
	var sb strings.Builder
	for _, elem := range elems {
		sb.WriteString(elem.expandOneString())
	}
	return sb.String()
}

func appendWord(w1, w2 word) word {
	if len(w1) > 0 && len(w2) > 0 && w1[len(w1)-1].text != "" && w2[0].text != "" {
		w1[len(w1)-1].text += w2[0].text
		w2 = w2[1:]
	}
	return append(w1, w2...)
}

func parseGlob(fields []string) []word {
	ts := make([]word, len(fields))
	for i, s := range fields {
		ts[i] = parseGlobOne(s)
	}
	return ts
}

func parseGlobOne(s string) word {
	var segs []wordSegment
	for s != "" {
		i := strings.IndexAny(s, "[]?*")
		if i == -1 {
			segs = append(segs, wordSegment{text: s})
			break
		}
		if i > 0 {
			segs = append(segs, wordSegment{text: s[:i]})
		}
		segs = append(segs, wordSegment{meta: s[i]})
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
		if s == "" {
			// Unquoted null words are deleted even with an empty IFS.
			return nil
		}
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
	fields := regexp.MustCompile(strings.Join(delimPatterns, "|")).Split(s, -1)
	if len(fields) > 0 && fields[len(fields)-1] == "" {
		// If the word ended with a delimiter, don't produce a final empty
		// field. See posix-ext/2.6.5-field-splitting.test.sh for details.
		//
		// This also implements the deletion of words that expand to exactly one
		// null field (see posix/2.6-word-expansion.test.sh).
		fields = fields[:len(fields)-1]
	}
	return fields
}
