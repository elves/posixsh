package eval

import (
	"regexp"
	"strings"

	"src.elv.sh/pkg/glob"
)

// Converts a [word] to a regexp pattern.
//
// This function is somewhat similar to [globPatternFromWord], but they don't
// share the implementation for two reasons:
//
//   - They convert a word to different types. Filename generation and pattern
//     matching are different tasks.
//
//   - The parsing of character class is different: when parsing a word for
//     filename generation, [...] must not contain any /. This restriction does
//     not apply when parsing a word for pattern matching.
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
				names = append(names, stringifySegs(w))
			} else {
				p.Glob(func(info glob.PathInfo) bool {
					names = append(names, info.Path)
					return true
				})
			}
		}
	}
	return names
}

// Converts a [word] to a [glob.Pattern]. Also returns whether any metacharacter
// has been parsed - "?", "*" and "[" that is successfully matched.
func globPatternFromWord(w word) (glob.Pattern, bool) {
	var globSegs []glob.Segment
	hasMeta := false
	// Split the word by slashes, and process the components separately. The
	// splitting needs to be done before character classes are parsed, as
	// specified by POSIX in 2.13.3 "Patterns used for filename expansion".
	for i, w := range splitWordBySlashes(w) {
		if i > 0 {
			globSegs = append(globSegs, glob.Slash{})
		}
		parsePattern(w, parsePatternFuncs{
			anyChar: func() {
				globSegs = append(globSegs, glob.Wild{Type: glob.Question})
				hasMeta = true
			},
			charClass: func(re *regexp.Regexp) {
				globSegs = append(globSegs, glob.Wild{
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
				globSegs = append(globSegs, glob.Wild{Type: glob.Star})
				hasMeta = true
			},
			literal: func(s string) {
				if n := len(globSegs); n > 0 && glob.IsLiteral(globSegs[n-1]) {
					globSegs[n-1] = glob.Literal{
						Data: globSegs[n-1].(glob.Literal).Data + s}
				} else {
					globSegs = append(globSegs, glob.Literal{Data: s})
				}
			},
		})
	}
	return glob.Pattern{Segments: globSegs}, hasMeta
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

func stringifySegs(segs []wordSegment) string {
	var sb strings.Builder
	for _, seg := range segs {
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
	// Pattern to match ASCII character class inside [ ].
	asciiCharClass = regexp.MustCompile(`^\[:[^]*]:\]`)
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
	// end[i] == {x, y} means that the i-th top-level "[" is matched to the
	// "]" at w[i].text[j]. If i >= len(end), it is unmatched.
	var end [][2]int
	// Whether we have seen a top-level "[" that is not yet matched.
	lbActive := false
	for i, seg := range w {
		if seg.quoted {
			continue
		}
		// Iterate by bytes: we are only looking for [ and ], so we don't
		// have to pay the overhead of UTF-8 decoding.
		j := 0
		for j < len(seg.text) {
			switch seg.text[j] {
			case '[':
				if !lbActive {
					lbActive = true
					j++
					// Skip over a ] or !] immediately after a top-level [.
					if j < len(seg.text) && seg.text[j] == '!' {
						j++
					}
					if j < len(seg.text) && seg.text[j] == ']' {
						j++
					}
				} else {
					// There's already an open [. Follow the rule of the
					// regexp package: only treat it as special if it forms
					// an ASCII character class; otherwise it's literal.
					if s := asciiCharClass.FindString(seg.text[j:]); s != "" {
						// Skip to after the closing ].
						j += len(s)
					} else {
						j++
					}
				}
			case ']':
				if lbActive {
					end = append(end, [2]int{i, j})
					lbActive = false
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
	lbSeq := 0
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
				iend, jend := -1, -1
				if lbSeq < len(end) {
					iend, jend = end[lbSeq][0], end[lbSeq][1]
					lbSeq++
				}
				if iend == -1 {
					// Unmatched "[" is literal text.
					j++
				} else {
					// Add the literal text part before "[".
					literal(w[i].text[jstart:j])
					// Collect the part surrounded by [ ].
					var content word
					if iend == i {
						// [ and ] are in the same segment.
						content = wordOfOneSeg(w[i].text[j+1:jend], w[i].quoted)
					} else {
						content = make(word, 0, iend-i+1)
						// Add the part of w[i] after "[".
						content = append(content, unquotedWord(w[i].text[j+1:])...)
						// Add internal segments.
						content = append(content, w[i+1:iend]...)
						// Add the part of w[iend] before "]".
						content = append(content, unquotedWord(w[iend].text[:jend])...)
					}
					// Build a regexp for the character class from the part
					// surrounded by [ and ].
					var sb strings.Builder
					sb.WriteString("^[")
					if len(content) > 0 && !content[0].quoted && strings.HasPrefix(content[0].text, "!") {
						// An unquoted leading ! means negation.
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
					sb.WriteString("]$")
					re, err := regexp.Compile(sb.String())
					if err != nil {
						// Invalid character class: treat as literal text.
						literal(sb.String())
					} else {
						f.charClass(re)
					}
					// Jump to after the "]".
					i, j = iend, jend+1
					continue fori
				}
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
