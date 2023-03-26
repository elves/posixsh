package eval

import (
	"bytes"
	"io"
)

var builtins = map[string]func(*frame, []string) int{
	"read": read,
	"set":  set,
}

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

// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#set
func set(fm *frame, args []string) int {
	// TODO: Support outputting parameters.
	// TODO: Support setting options.
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	*fm.arguments = append([]string{(*fm.arguments)[0]}, args...)
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
