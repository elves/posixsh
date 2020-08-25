package parse

import "strings"

// String utilities.

func hasPrefix(s, p string) bool {
	return strings.HasPrefix(s, p)
}
