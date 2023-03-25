package parse

import (
	"fmt"
	"strings"
)

type Error struct {
	Errors []ErrorEntry
}

func (err Error) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%v parseNoOpt errors: ", len(err.Errors))
	for i, e := range err.Errors {
		if i > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "%v: %v", e.Position, e.Message)
	}
	return b.String()
}

type ErrorEntry struct {
	Position int
	Message  string
}
