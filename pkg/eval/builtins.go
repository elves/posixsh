package eval

import (
	"bytes"
	"io"
)

var builtins = map[string]func(*frame, []string) int{
	"alias":   alias,
	"bg":      bg,
	"cd":      cd,
	"false":   falseCmd,
	"fc":      fc,
	"fg":      fg,
	"getopts": getopts,
	"hash":    hash,
	"jobs":    jobs,
	"kill":    kill,
	"newgrp":  newgrp,
	"pwd":     pwd,
	"read":    read,
	"true":    trueCmd,
	"umask":   umask,
	"unalias": unalias,
	"wait":    wait,
}

func alias(fm *frame, args []string) int {
	// TODO
	return 0
}

func bg(fm *frame, args []string) int {
	// TODO
	return 0
}

func cd(fm *frame, args []string) int {
	// TODO
	return 0
}

func falseCmd(*frame, []string) int { return 1 }

func fc(fm *frame, args []string) int {
	// TODO
	return 0
}

func fg(fm *frame, args []string) int {
	// TODO
	return 0
}

func getopts(fm *frame, args []string) int {
	// TODO
	return 0
}

func hash(fm *frame, args []string) int {
	// TODO
	return 0
}

func jobs(fm *frame, args []string) int {
	// TODO
	return 0
}

func kill(fm *frame, args []string) int {
	// TODO
	return 0
}

func newgrp(fm *frame, args []string) int {
	// TODO
	return 0
}

func pwd(fm *frame, args []string) int {
	// TODO
	return 0
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

func umask(fm *frame, args []string) int {
	// TODO
	return 0
}

func unalias(fm *frame, args []string) int {
	// TODO
	return 0
}

func wait(fm *frame, args []string) int {
	// TODO
	return 0
}
