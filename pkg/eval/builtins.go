package eval

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var builtins = map[string]func(*frame, []string) int{
	"alias":   aliasCmd,
	"bg":      bgCmd,
	"cd":      cdCmd,
	"false":   falseCmd,
	"fc":      fcCmd,
	"fg":      fgCmd,
	"getopts": getoptsCmd,
	"hash":    hashCmd,
	"jobs":    jobsCmd,
	"kill":    killCmd,
	"newgrp":  newgrpCmd,
	"pwd":     pwdCmd,
	"read":    readCmd,
	"true":    trueCmd,
	"type":    typeCmd,
	"ulimit":  ulimitCmd,
	"umask":   umaskCmd,
	"unalias": unaliasCmd,
	"wait":    waitCmd,
}

func aliasCmd(fm *frame, args []string) int {
	// TODO
	return 0
}

func bgCmd(fm *frame, args []string) int {
	// TODO
	return 0
}

const pathSep = string(filepath.Separator)

func cdCmd(fm *frame, args []string) int {
	opts, args, err := getopts(args, "LP")
	if err != nil {
		fm.badCommandLine("%v", err)
		return StatusBadCommandLine
	}
	if len(args) == 0 {
		return cdInner(fm, fm.GetVar("HOME"))
	} else if len(args) > 1 {
		fm.badCommandLine("cd accepts at most one argument")
		return StatusBadCommandLine
	}

	logical := true
	// POSIX requires that the option that appears later take precedence.
	for _, opt := range opts {
		logical = opt.name == 'L'
	}
	newWd := args[0]
	if newWd == "-" {
		status := cdInner(fm, fm.GetVar("OLDPWD"))
		if status == 0 {
			fmt.Fprintln(fm.files[1], fm.wd)
		}
		return status
	}
	if !filepath.IsAbs(newWd) {
		if first, _, _ := strings.Cut(newWd, "/"); first != "." && first != ".." {
			for _, cdpath := range filepath.SplitList(fm.GetVar("CDPATH")) {
				// See if we can change to cdpath + newWd. This duplicates some
				// code from below.
				tryWd := cdpath + string(filepath.Separator) + newWd
				if !filepath.IsAbs(tryWd) {
					tryWd = fm.wd + pathSep + tryWd
				}
				if logical {
					tryWd = filepath.Clean(tryWd)
				}
				if info, err := os.Stat(tryWd); err == nil && info.IsDir() {
					return cdNoCheck(fm, tryWd)
				}
			}
		}
		// Don't use [filepath.Join] as it always calls [filepath.Clean]. The
		// path will eventually be cleaned by [filepath.EvalSymlinks].
		newWd = fm.wd + pathSep + newWd
	}
	if logical {
		newWd = filepath.Clean(newWd)
	}
	return cdInner(fm, newWd)
}

func cdInner(fm *frame, newWd string) int {
	info, err := os.Stat(newWd)
	if err != nil {
		fmt.Fprintf(fm.files[2], "cannot cd to %v: %v", newWd, err)
		return 2
	}
	if !info.IsDir() {
		fmt.Fprintf(fm.files[2], "cannot cd to %v as it is not a directory", newWd)
		return 2
	}
	return cdNoCheck(fm, newWd)
}

func cdNoCheck(fm *frame, newWd string) int {
	newWd, err := filepath.EvalSymlinks(newWd)
	if err != nil {
		fmt.Fprintf(fm.files[2], "cannot cd to %v: %v", newWd, err)
		return 2
	}
	// POSIX doesn't specify whether cd should respect the readonly attribute of
	// $OLDPWD and $PWD; bash, dash and zsh do, ksh doesn't. We follow ksh.
	fm.variables.values["OLDPWD"] = fm.GetVar("PWD")
	fm.variables.values["PWD"] = newWd
	fm.wd = newWd
	return 0
}

func falseCmd(*frame, []string) int { return 1 }

func fcCmd(fm *frame, args []string) int {
	// TODO
	return 0
}

func fgCmd(fm *frame, args []string) int {
	// TODO
	return 0
}

func getoptsCmd(fm *frame, args []string) int {
	// TODO
	return 0
}

func hashCmd(fm *frame, args []string) int {
	// TODO
	return 0
}

func jobsCmd(fm *frame, args []string) int {
	// TODO
	return 0
}

func killCmd(fm *frame, args []string) int {
	// TODO
	return 0
}

func newgrpCmd(fm *frame, args []string) int {
	// TODO
	return 0
}

func pwdCmd(fm *frame, args []string) int {
	opts, args, err := getopts(args, "LP")
	if err != nil {
		fm.badCommandLine("%v", err)
		return StatusBadCommandLine
	}
	if len(args) > 0 {
		fm.badCommandLine("pwd doesn't accept arguments")
		return StatusBadCommandLine
	}
	logical := true
	// POSIX requires that the option that appears later take precedence.
	for _, opt := range opts {
		logical = opt.name == 'L'
	}

	if logical {
		fmt.Fprintln(fm.files[1], fm.wd)
	} else {
		wd, err := filepath.EvalSymlinks(fm.wd)
		if err != nil {
			fm.badCommandLine("cannot resolve working directory: %v", err)
			return 2
		}
		fmt.Fprintln(fm.files[1], wd)
	}
	return 0
}

func readCmd(fm *frame, args []string) int {
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

func ulimitCmd(fm *frame, args []string) int {
	// TODO
	return 0
}

func umaskCmd(fm *frame, args []string) int {
	// TODO
	return 0
}

func unaliasCmd(fm *frame, args []string) int {
	// TODO
	return 0
}

func waitCmd(fm *frame, args []string) int {
	// TODO
	return 0
}
