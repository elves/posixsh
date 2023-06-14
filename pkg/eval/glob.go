package eval

import (
	"strings"

	"src.elv.sh/pkg/glob"
)

func (fm *frame) glob(words []globWord) []string {
	var names []string
	for _, word := range words {
		names = fm.globOne(names, word)
	}
	return names
}

func (fm *frame) globOne(names []string, word globWord) []string {
	if len(word) == 0 {
		// Empty word. This can result from field splitting, for example.
		return append(names, "")
	}
	if len(word) == 1 && word[0].meta == 0 {
		// Word with no glob metacharacters. Because the code that builds
		// globWord always merges neighboring text segments, such words always
		// have exactly one text segment.
		return append(names, word[0].text)
	}
	convertGlobWord(word).Glob(func(info glob.PathInfo) bool {
		names = append(names, info.Path)
		return true
	})
	return names
}

func convertGlobWord(word globWord) glob.Pattern {
	var segs []glob.Segment
	appendLiteral := func(s string) {
		if len(segs) > 0 && glob.IsLiteral(segs[len(segs)-1]) {
			segs[len(segs)-1] = glob.Literal{
				Data: segs[len(segs)-1].(glob.Literal).Data + s}
		} else {
			segs = append(segs, glob.Literal{Data: s})
		}
	}
	for i := 0; i < len(word); i++ {
		switch word[i].meta {
		case '[':
			for j := i + 1; j < len(word); j++ {
				if word[j].meta == ']' {
					matcher := convertCharClass(word[i+1 : j])
					segs = append(segs, glob.Wild{
						Type: glob.Question, Matchers: []func(rune) bool{matcher}})
					i = j
					break
				}
				if strings.Contains(word[j].text, "/") {
					break
				}
			}
			appendLiteral("[")
		case ']':
			appendLiteral("]")
		case '?':
			segs = append(segs, glob.Wild{Type: glob.Question})
		case '*':
			segs = append(segs, glob.Wild{Type: glob.Star})
		default:
			s := word[i].text
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

func convertCharClass(segs []globWordSegment) func(rune) bool {
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
