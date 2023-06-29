package eval

import (
	"regexp"
	"strings"
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
	var sb strings.Builder
	sb.WriteString("(?s)") // let . match \n
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
						sb.WriteString(convertCharClassToRegexp(stringifySegs(w[i+1 : j])))
						i = j
						parsedCharClass = true
						break
					}
				}
			}
			if !parsedCharClass {
				sb.WriteString(`\[`)
			}
		case ']':
			// If we reached here, the ] was not matched by a [.
			sb.WriteString(`\]`)
		case '?':
			sb.WriteByte('.')
		case '*':
			if shortest {
				sb.WriteString(".*?")
			} else {
				sb.WriteString(".*")
			}
		default:
			sb.WriteString(regexp.QuoteMeta(w[i].text))
		}
	}
	return sb.String()
}

func convertCharClassToRegexp(s string) string {
	// TODO: Support range and class
	if strings.HasPrefix(s, "!") {
		return "[^" + regexp.QuoteMeta(s[1:]) + "]"
	}
	return "[" + regexp.QuoteMeta(s) + "]"
}
