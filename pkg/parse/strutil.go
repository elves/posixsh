package parse

import "strings"

// String utilities.

func hasPrefix(s, p string) bool {
	return strings.HasPrefix(s, p)
}

func runeIn(r rune, set string) bool {
	return strings.ContainsRune(set, r)
}
