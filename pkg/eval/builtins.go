package eval

import (
	"bytes"
	"io"
)

// Some builtins are designated "special" by POSIX; the return value includes a
// bool because they can return a fatal error that terminates evaluation.
//
// For more details on how special builtins differ from non-special builtins,
// see the code that uses this map.
var specialBuiltins = map[string]func(*frame, []string) (int, bool){
	":":   colon,
	"set": set,
}

func colon(*frame, []string) (int, bool) {
	return 0, true
}

// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#set
func set(fm *frame, args []string) (int, bool) {
	// TODO: Support outputting parameters.
	// TODO: Support setting options.
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	fm.arguments = append([]string{fm.arguments[0]}, args...)
	return 0, true
}

var builtins = map[string]func(*frame, []string) int{
	"false": falseCmd,
	"read":  read,
	"true":  trueCmd,
}

func falseCmd(*frame, []string) int { return 1 }

func read(fm *frame, args []string) int {
	line := getLine(fm.files[0])
	if len(args) == 0 {
		fm.variables["REPLY"] = line
	} else {
		fm.variables[args[0]] = line
		// TODO: Support multiple arguments:
	}
	return 0
}

func getLine(r io.Reader) string {
	var buf bytes.Buffer
	for {
		var buf1 [1]byte
		nr, err := r.Read(buf1[:])
		if nr == 0 || err != nil || buf1[0] == '\n' {
			break
		}
		buf.WriteByte(buf1[0])
	}
	return buf.String()
}

func trueCmd(*frame, []string) int { return 0 }
