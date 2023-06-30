package eval

import (
	"regexp"
	"strings"

	"src.elv.sh/pkg/glob"
)

// Converts a [word] to a regexp pattern. This doesn't compile the pattern so
// that the caller can use the result as part of a bigger pattern.
func regexpPatternFromWord(w word, shortest bool) string {
	anyString := ".*"
	if shortest {
		anyString = ".*?"
	}
	var sb strings.Builder
	sb.WriteString("(?s)") // let . match \n
	parsePattern(w, parsePatternFuncs{
		anyChar:   func() { sb.WriteByte('.') },
		charClass: func(re *regexp.Regexp) { sb.WriteString(re.String()) },
		anyString: func() { sb.WriteString(anyString) },
		literal:   func(s string) { sb.WriteString(regexp.QuoteMeta(s)) },
	})
	return sb.String()
}

// Performs filename generation on each word.
func generateFilenames(words []word) []string {
	var names []string
	for _, w := range words {
		if len(w) == 0 {
			// Empty word. This can be an empty string literal or an empty field
			// after field splitting.
			names = append(names, "")
		} else if len(w) == 1 && w[0].quoted {
			// Quoted word; add directly.
			names = append(names, w[0].text)
		} else {
			p, hasMeta := globPatternFromWord(w)
			if !hasMeta {
				// Word is unquoted or partially unquoted, but no globbing
				// metacharacter was parsed; add directly.
				names = append(names, stringifyWord(w))
			} else {
				hasMatch := false
				p.Glob(func(info glob.PathInfo) bool {
					names = append(names, info.Path)
					hasMatch = true
					return true
				})
				if !hasMatch {
					// POSIX requires that patterns with no matches be treated
					// as literal text.
					names = append(names, stringifyWord(w))
				}
			}
		}
	}
	return names
}

// Converts a [word] to a [glob.Pattern]. Also returns whether any metacharacter
// has been parsed - "?", "*" and "[" that is successfully matched.
func globPatternFromWord(w word) (glob.Pattern, bool) {
	var segs []glob.Segment
	hasMeta := false
	// Split the word by slashes, and process the components separately. The
	// splitting needs to be done before character classes are parsed, as
	// specified by POSIX in 2.13.3 "Patterns used for filename expansion".
	for i, w := range splitWordBySlashes(w) {
		if i > 0 {
			segs = append(segs, glob.Slash{})
		}
		parsePattern(w, parsePatternFuncs{
			anyChar: func() {
				segs = append(segs, glob.Wild{Type: glob.Question})
				hasMeta = true
			},
			charClass: func(re *regexp.Regexp) {
				segs = append(segs, glob.Wild{
					Type: glob.Question,
					Matchers: []func(rune) bool{
						func(r rune) bool {
							return re.MatchString(string(r))
						},
					},
				})
				hasMeta = true
			},
			anyString: func() {
				segs = append(segs, glob.Wild{Type: glob.Star})
				hasMeta = true
			},
			literal: func(s string) {
				if n := len(segs); n > 0 && glob.IsLiteral(segs[n-1]) {
					segs[n-1] = glob.Literal{
						Data: segs[n-1].(glob.Literal).Data + s}
				} else {
					segs = append(segs, glob.Literal{Data: s})
				}
			},
		})
	}
	return glob.Pattern{Segments: segs}, hasMeta
}

// Splits a word by slashes, whether quoted or unquoted.
func splitWordBySlashes(w word) []word {
	var words []word
	for _, seg := range w {
		parts := strings.Split(seg.text, "/")
		// Add the first part to the previous word, creating the first word if
		// necessary.
		if len(words) == 0 {
			words = []word{wordOfOneSeg(parts[0], seg.quoted)}
		} else if parts[0] != "" {
			pw := &words[len(words)-1]
			*pw = append(*pw, wordSegment{parts[0], seg.quoted})
		}
		// Add the remaining parts as separate words.
		for _, part := range parts[1:] {
			words = append(words, wordOfOneSeg(part, seg.quoted))
		}
	}
	return words
}

func stringifyWord(w word) string {
	var sb strings.Builder
	for _, seg := range w {
		sb.WriteString(seg.text)
	}
	return sb.String()
}

type parsePatternFuncs struct {
	literal   func(string)
	anyChar   func()
	charClass func(*regexp.Regexp)
	anyString func()
}

var (
	// Pattern to match ASCII character class inside [ ]. The set of supported
	// classes are from item 6 of section 9.3.5 "RE bracket expression" of
	// POSIX. These classes are all supported by the regexp package.
	asciiCharClass = regexp.MustCompile(`^\[:(?:alnum|alpha|blank|cntrl|digit|graph|lower|print|punct|space|upper|xdigit):\]`)
	// Characters that can be special inside [ ].
	bracketSpecial = regexp.MustCompile(`[\[\]\-\\^:]`)
)

func parsePattern(w word, f parsePatternFuncs) {
	literal := func(s string) {
		if s != "" {
			f.literal(s)
		}
	}
	// Match all the unquoted [ ] pairs first.
	//
	// end[{i, j}] = {x, y} means that the "[" at w[i].text[j] is matched to the
	// "]" at w[x].text[y]. If the key is not found, the "[" is unmatched.
	end := make(map[[2]int][2]int)
	// The start position of the unclosed top-level "[".
	lbStart := [2]int{-1, -1}
	for i, seg := range w {
		if seg.quoted {
			continue
		}
		j := 0
		for j < len(seg.text) {
			switch seg.text[j] {
			case '[':
				if lbStart[0] == -1 {
					// Top-level [.
					lbStart = [2]int{i, j}
					j++
					// Skip over ] or !] immediately after a top-level [.
					if j < len(seg.text) && seg.text[j] == '!' {
						j++
					}
					if j < len(seg.text) && seg.text[j] == ']' {
						j++
					}
				} else {
					// There's already an open [. All of dash, bash and ksh
					// follow similar rules: only treat it as special if it
					// forms an ASCII character class; otherwise it's literal.
					// This is also the rule used by the regexp package, so
					// leaving the [ unescaped when building the regular
					// expression is fine.
					//
					// However, the behavior of dash, bash and ksh differ when
					// the part after [ looks like an ASCII character class but
					// has invalid class name (like [:bad:]):
					//
					//  - Dash treats the [ as literal, and the next ] will
					//    close the outer [.
					//
					//  - Bash still matches the next ] to the [. If the outer [
					//    is eventually closed, the [:bad:] part results in an
					//    empty character class. If the outer [ is not closed,
					//    the [:bad:] part is treated as a bracket expression
					//    itself (matching : b a d).
					//
					//  - Ksh gives up and makes the part from the outer [ to
					//    this point literal. This behavior is allowed by POSIX
					//    section 2.13.3.
					//
					// We follow dash's behavior, since it is more sensible than
					// ksh's and slightly easier to implement than bash's.
					//
					// POSIX doesn't specify whether the ASCII character class
					// must be unquoted. Both dash and bash require it to be
					// unquoted; ksh and zsh don't. We follow the former
					// behavior since it's easier to implement.
					if s := asciiCharClass.FindString(seg.text[j:]); s != "" {
						// Record the range of inner [ ]: this will be used if
						// the outer [ is unmatched. In that case, the outer [
						// becomes a literal [, and the part the inner [ ]
						// becomes be a top-level bracket expression.
						end[[2]int{i, j}] = [2]int{i, j + len(s) - 1}
						// Skip to after the closing ].
						j += len(s)
					} else {
						j++
					}
				}
			case ']':
				if lbStart[0] != -1 {
					end[lbStart] = [2]int{i, j}
					lbStart = [2]int{-1, -1}
				}
				j++
			default:
				j++
			}
		}
	}
	// Convert the word into glob segments. Iterating through the word and
	// its segments in a single loop simplifies the parsing of character
	// classes, which can jump to the middle of another segment.
	i, j := 0, 0
fori:
	for i < len(w) {
		if j == len(w[i].text) {
			i, j = i+1, 0
			continue
		}
		if w[i].quoted {
			literal(w[i].text[j:])
			i, j = i+1, 0
			continue
		}
		// Keep track of the start of a literal segment.
		jstart := j
		for j < len(w[i].text) {
			switch w[i].text[j] {
			case '[':
				endPos, matched := end[[2]int{i, j}]
				if !matched {
					// Unmatched "[" is literal text; keep going.
					j++
					continue
				}
				// Add the literal text part before "[".
				literal(w[i].text[jstart:j])
				// Convert the character class to regular expression.
				iend, jend := endPos[0], endPos[1]
				charClass := convertCharClassToRegexp(w, i, j, iend, jend)
				re, err := regexp.Compile(charClass)
				if err != nil {
					// This shouldn't happen we have made sure that the ASCII
					// character classes are valid, but there might be other
					// possible syntactical errors inside brackets. Fall back to
					// literal text.
					literal(charClass)
				} else {
					f.charClass(re)
				}
				// Jump to after the "]".
				i, j = iend, jend+1
				continue fori
			case '?', '*':
				// Add the literal text part before the metacharacter.
				literal(w[i].text[jstart:j])
				if w[i].text[j] == '?' {
					f.anyChar()
				} else {
					f.anyString()
				}
				j++
				continue fori
			default:
				j++
			}
		}
		// If we reached here, we have reached the end of w[i] without
		// parsing a metacharacter.
		literal(w[i].text[jstart:j])
	}
}

func convertCharClassToRegexp(w word, i0, j0, iend, jend int) string {
	var content word
	if iend == i0 {
		// [ and ] are in the same segment.
		content = wordOfOneSeg(w[i0].text[j0+1:jend], w[i0].quoted)
	} else {
		content = make(word, 0, iend-i0+1)
		// Add the part of w[i] after "[".
		content = append(content, unquotedWord(w[i0].text[j0+1:])...)
		// Add internal segments.
		content = append(content, w[i0+1:iend]...)
		// Add the part of w[iend] before "]".
		content = append(content, unquotedWord(w[iend].text[:jend])...)
	}
	var sb strings.Builder
	sb.WriteString("[")
	if len(content) > 0 && !content[0].quoted && strings.HasPrefix(content[0].text, "!") {
		// Turn an unquoted leading ! to ^.
		sb.WriteByte('^')
		content[0].text = content[0].text[1:]
	}
	for _, seg := range content {
		if seg.quoted {
			// Escape anything that can be special inside
			// character classes. We can't use
			// regexp.QuoteMeta here since some characters
			// like "-" are only special inside character
			// classes.
			sb.WriteString(bracketSpecial.ReplaceAllString(seg.text, "\\$1"))
		} else {
			sb.WriteString(seg.text)
		}
	}
	sb.WriteString("]")
	return sb.String()
}
