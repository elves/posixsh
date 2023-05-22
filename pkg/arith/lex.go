package arith

// Token predicates.
func isNumberToken(s string) bool   { return isNumber(s[0]) }
func isVariableToken(s string) bool { return isLetter(s[0]) }

// Breaks arithmetic expression into tokens.
func lex(s string) ([]string, []int) {
	i := 0
	var tokens []string
	var starts []int
	for i < len(s) {
		start := i
		if isWhitespace(s[i]) {
			// Skip whitespaces
			i++
			for i < len(s) && isWhitespace(s[i]) {
				i++
			}
			continue
		}
		if isLetter(s[i]) || isNumber(s[i]) {
			// Lump number and variable tokens together, since numbers can
			// contain letters (like 0x10) and variables can contain numbers
			// (like a1). They can be later distinguished with isNumberToken and
			// isVariableToken.
			i++
			for i < len(s) && (isLetter(s[i]) || isNumber(s[i])) {
				i++
			}
		} else {
			if i+1 < len(s) && isTwoByteOp(s[i:i+2]) {
				i += 2
			} else {
				i += 1
			}
		}
		tokens = append(tokens, s[start:i])
		starts = append(starts, start)
	}
	return tokens, starts
}

func isWhitespace(x byte) bool { return x == ' ' || x == '\t' || x == '\r' || x == '\n' || x == '\v' }

// Note: _ is considered a letter.
func isLetter(x byte) bool { return 'a' <= x && x <= 'z' || 'A' <= x && x <= 'Z' || x == '_' }
func isNumber(x byte) bool { return '0' <= x && x <= '9' }

func isTwoByteOp(s string) bool {
	switch s {
	case "||", "&&", "==", "!=", "<=", ">=", "<<", ">>", "++", "--":
		// Note: For simplicity, augmented assignment operators are parsed as
		// two separate operators. This allows expressions like "a + = 2", which
		// is not valid syntax in other shells, but it's harmless to support it.
		return true
	}
	return false
}
