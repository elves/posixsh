package eval

import (
	"strings"

	"src.elv.sh/pkg/glob"
)

func (fm *frame) glob(words []word) []string {
	var names []string
	for _, w := range words {
		names = fm.globOne(names, w)
	}
	return names
}

// Perform pathname expansion on one word, appending to the names.
func (fm *frame) globOne(names []string, w word) []string {
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
	p := convertGlobWord(w)
	if len(p.Segments) == 1 && glob.IsLiteral(p.Segments[0]) {
		// [ and ] may be "downgraded" to normal characters when they can't form
		// a valid character class, so it's possible that the word is literal
		// text after all.
		//
		// TODO: Also handle the case where some slashes have been parsed to
		// glob.Slash.
		return append(names, p.Segments[0].(glob.Literal).Data)
	}
	p.Glob(func(info glob.PathInfo) bool {
		names = append(names, info.Path)
		return true
	})
	return names
}

func convertGlobWord(w word) glob.Pattern {
	var segs []glob.Segment
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
			parsedCharClass := false
			for j := i + 1; j < len(w); j++ {
				if w[j].meta == ']' {
					matcher := convertCharClass(w[i+1 : j])
					segs = append(segs, glob.Wild{
						Type: glob.Question, Matchers: []func(rune) bool{matcher}})
					i = j
					parsedCharClass = true
					break
				}
				if strings.Contains(w[j].text, "/") {
					break
				}
			}
			if !parsedCharClass {
				appendLiteral("[")
			}
		case ']':
			appendLiteral("]")
		case '?':
			segs = append(segs, glob.Wild{Type: glob.Question})
		case '*':
			segs = append(segs, glob.Wild{Type: glob.Star})
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
	return glob.Pattern{Segments: segs}
}

func convertCharClass(segs []wordSegment) func(rune) bool {
	var sb strings.Builder
	for _, seg := range segs {
		if seg.meta != 0 {
			sb.WriteByte(seg.meta)
		} else {
			sb.WriteString(seg.text)
		}
	}
	// TODO: Support range and class
	s := sb.String()
	if strings.HasPrefix(s, "!") {
		s = s[1:]
		return func(r rune) bool { return !strings.ContainsRune(s, r) }
	}
	return func(r rune) bool { return strings.ContainsRune(s, r) }
}
