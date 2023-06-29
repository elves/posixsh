package eval

import (
	"strings"

	"src.elv.sh/pkg/glob"
)

func generateFilenames(words []word) []string {
	var names []string
	for _, w := range words {
		names = appendFilenames(names, w)
	}
	return names
}

// Perform pathname expansion on one word, appending to the names.
func appendFilenames(names []string, w word) []string {
	if len(w) == 0 {
		// Empty word. This can result from field splitting, for example.
		return append(names, "")
	}
	if len(w) == 1 && w[0].meta == 0 {
		// Word with no glob metacharacters. Because the code that builds
		// globWord always merges neighboring text segments, such words always
		// have exactly one text segment.
		return append(names, w[0].text)
	}
	p, hasMeta := globPatternFromWord(w)
	if !hasMeta {
		// [ and ] may be "downgraded" to normal characters when they can't form
		// a valid character class, so it's possible that the word is literal
		// text after all.
		return append(names, stringifySegs(w))
	}
	p.Glob(func(info glob.PathInfo) bool {
		names = append(names, info.Path)
		return true
	})
	return names
}

// Converts a [word] to a [glob.Pattern]. Also returns whether any metacharacter
// has been parsed - "?", "*" and "[" that is successfully matched.
func globPatternFromWord(w word) (glob.Pattern, bool) {
	var segs []glob.Segment
	hasMeta := false
	appendLiteral := func(s string) {
		if len(segs) > 0 && glob.IsLiteral(segs[len(segs)-1]) {
			segs[len(segs)-1] = glob.Literal{
				Data: segs[len(segs)-1].(glob.Literal).Data + s}
		} else {
			segs = append(segs, glob.Literal{Data: s})
		}
	}
	for i := 0; i < len(w); i++ {
		switch w[i].meta {
		case '[':
			// TODO: O(n^2) if there are a lot of unmatched ['s.
			parsedCharClass := false
			unmatchedLeftBracket := 1
			for j := i + 1; j < len(w); j++ {
				if w[j].meta == '[' {
					unmatchedLeftBracket++
				} else if w[j].meta == ']' {
					if j == i+1 || j == i+2 && w[j-1].text == "!" {
						// A ] directly after [ or [! is not special.
						continue
					}
					unmatchedLeftBracket--
					if unmatchedLeftBracket == 0 {
						matcher := convertCharClassToPredicate(stringifySegs(w[i+1 : j]))
						segs = append(segs, glob.Wild{
							Type: glob.Question, Matchers: []func(rune) bool{matcher}})
						i = j
						hasMeta = true
						parsedCharClass = true
						break
					}
				} else if strings.Contains(w[j].text, "/") {
					break
				}
			}
			if !parsedCharClass {
				appendLiteral("[")
			}
		case ']':
			// If we reached here, the ] was not matched by a [.
			appendLiteral("]")
		case '?':
			segs = append(segs, glob.Wild{Type: glob.Question})
			hasMeta = true
		case '*':
			segs = append(segs, glob.Wild{Type: glob.Star})
			hasMeta = true
		default:
			s := w[i].text
			for s != "" {
				i := strings.IndexByte(s, '/')
				if i == -1 {
					appendLiteral(s)
					break
				}
				if i > 0 {
					appendLiteral(s[:i])
				}
				segs = append(segs, glob.Slash{})
				s = s[i+1:]
			}
		}
	}
	return glob.Pattern{Segments: segs}, hasMeta
}

func convertCharClassToPredicate(s string) func(rune) bool {
	// TODO: Support range and class
	if strings.HasPrefix(s, "!") {
		s = s[1:]
		return func(r rune) bool { return !strings.ContainsRune(s, r) }
	}
	return func(r rune) bool { return strings.ContainsRune(s, r) }
}

func stringifySegs(segs []wordSegment) string {
	var sb strings.Builder
	for _, seg := range segs {
		if seg.meta != 0 {
			sb.WriteByte(seg.meta)
		} else {
			sb.WriteString(seg.text)
		}
	}
	return sb.String()
}
