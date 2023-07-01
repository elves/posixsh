package eval

import (
	"bytes"
	"fmt"
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
	"type":    typeCmd,
	"ulimit":  ulimit,
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
	varName := "REPLY"
	if len(args) > 0 {
		varName = args[0]
		// TODO: Support multiple arguments:
	}
	canSet := fm.SetVar(varName, line)
	if !canSet {
		// TODO: Add range information
		fmt.Fprintf(fm.files[2], "%v is readonly\n", varName)
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

func typeCmd(fm *frame, args []string) int {
	// TODO
	return 0
}

func ulimit(fm *frame, args []string) int {
	// TODO
	return 0
}

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
