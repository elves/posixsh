package eval

import (
	"regexp"
	"strings"

	"github.com/elves/posixsh/pkg/parse"
)

// Consumes a prefix of zero or more nodes for alias substitution. Returns
// expanded words, and the remaining nodes that still need to be evaluated
// normally. The first return value is safe to mutate.
func expandAlias(nodes []*parse.Compound, aliases map[string]string) ([]string, []*parse.Compound) {
	return aliasExpander{aliases, make(set[string])}.expand(nodes)
}

type aliasExpander struct {
	aliases map[string]string
	active  set[string]
}

func (ae aliasExpander) expand(words []*parse.Compound) ([]string, []*parse.Compound) {
	return expandAliasNodes(ae, words, barewordNode)
}

// Directly implements [aliasExpander.expand], but is also a slightly more
// general version of expanding a string slice ([aliasExpander.expandStrings]).
func expandAliasNodes[N any](ae aliasExpander, words []N, f func(N) (string, bool)) ([]string, []N) {
	if len(words) == 0 {
		return nil, words
	}
	name, ok := f(words[0])
	if !ok || ae.active.has(name) {
		return nil, words
	}
	def, ok := ae.aliases[name]
	if !ok {
		return nil, words
	}
	head, next := parseAliasDef(def)
	ae.active.add(name)
	defer ae.active.del(name)
	head = expandAliasStrings(ae, head)
	tail := words[1:]
	if next {
		tailHead, tailTail := expandAliasNodes(ae, tail, f)
		return append(head, tailHead...), tailTail
	}
	return head, tail
}

// A special case of [expandAliasNodes]:
//
//   - Words are always eligible for alias expansion since they are just
//     strings.
//   - Head and tail are merged together.
//
// Used for recursively expanding aliases in [expandAliasNodes].
func expandAliasStrings(ae aliasExpander, words []string) []string {
	head, tail := expandAliasNodes(ae, words, func(s string) (string, bool) { return s, true })
	return append(head, tail...)
}

func barewordNode(n *parse.Compound) (string, bool) {
	if len(n.Parts) == 1 && n.Parts[0].Type == parse.BarewordPrimary {
		return n.Parts[0].Value, true
	}
	return "", false
}

// Reports whether an alias definition only has barewords with no globbing
// characters and doesn't start with an assignment.
func aliasSupported(def string) bool {
	words, _ := parseAliasDef(def)
	if len(words) == 0 {
		return true
	}
	if strings.Contains(words[0], "=") {
		return false
	}
	for _, word := range words {
		if strings.ContainsAny(word, nonBareword+"?*[") {
			return false
		}
	}
	return true
}

var tabsOrSpaces = regexp.MustCompile(`[ \t]+`)

func parseAliasDef(def string) ([]string, bool) {
	trimmed := strings.TrimRight(def, " \t")
	expandNext := trimmed != def
	def = strings.TrimLeft(trimmed, " \t")
	return tabsOrSpaces.Split(def, -1), expandNext
}
