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
	"alias": aliasCmd,
	"bg":    bgCmd,
	"cd":    cdCmd,
	// command is added in init
	"false":   falseCmd,
	"fc":      fcCmd,
	"fg":      fgCmd,
	"getopts": getoptsCmd,
	"hash":    hashCmd,
	"jobs":    jobsCmd,
	// kill and newgrp are omitted; they are usually available as external
	// commands.
	"pwd":  pwdCmd,
	"read": readCmd,
	"true": trueCmd,
	// type is added in init
	"ulimit":  ulimitCmd,
	"umask":   umaskCmd,
	"unalias": unaliasCmd,
	"wait":    waitCmd,
}

func init() {
	// Some commands are added in the map here to avoid dependency cycles.
	builtins["command"] = commandCmd
	builtins["type"] = typeCmd
}

// Limitation: the alias substitution mechanism specified by POSIX requires the
// results to participate in the grammar of the command. For example, the
// following is valid:
//
//	alias x='echo x; echo'
//	x bar # equivalent to: echo x; echo bar
//
// This is particularly difficult for this implementation as most of the parsing
// happens statically without the knowledge of the alias table. Instead, we only
// definitions that consist of barewords and returns an error when there is
// anything more complex.
func aliasCmd(fm *frame, args []string) int {
	if len(args) == 0 {
		printAliases(fm)
		return 0
	}
	status := 0
	for _, arg := range args {
		name, def, hasDef := strings.Cut(arg, "=")
		if hasDef {
			// POSIX doesn't specify the set of supported alias names, but since
			// only barewords are eligible for alias expansion, we only admit
			// names that consist of barewords and don't look like assignments.
			//
			// Since we parse glob characters as barewords too, we also support
			// them as alias names - for example, "alias '*'=echo" is supported.
			// This is consistent with dash, bash and zsh, but not with ksh.
			if strings.ContainsAny(name, nonBareword+"=") {
				fmt.Fprintf(fm.files[2], "alias name with metacharacters or '=' are not supported: %v", name)
				status = 1
				continue
			}
			if aliasSupported(def) {
				fm.aliases[name] = def
			} else {
				fmt.Fprintf(fm.files[2], "alias definitions with metacharacters are not supported: %v", def)
				status = 1
			}
		} else {
			if _, ok := fm.aliases[name]; ok {
				printAlias(fm, name)
			} else {
				fmt.Fprintf(fm.files[2], "no alias definitions for %v", name)
				status = 1
			}
		}
	}
	return status
}

func printAliases(fm *frame) {
	// POSIX doesn't requires the names to be sorted, but we do that to make the
	// output more readable.
	names := sortedNames(fm.aliases)
	for _, name := range names {
		printAlias(fm, name)
	}
}

func printAlias(fm *frame, name string) {
	// It would make more sense to prefix the output with "alias" so that it
	// could be executed as code, but this format is specified by POSIX.
	fmt.Fprintf(fm.files[1], "%v=%v\n", quote(name), quote(fm.aliases[name]))
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

const defaultPath = "/usr/bin:/bin:/usr/sbin:/sbin"

func commandCmd(fm *frame, args []string) int {
	opts, args, err := getopts(args, "pvV")
	if err != nil {
		fm.badCommandLine("%v", err)
		return StatusBadCommandLine
	}
	path := fm.GetVar("PATH")
	if opts.has('p') {
		path = defaultPath
	}
	// POSIX only requires that "command -v" and "command -V" support exactly
	// one argument, but all of bash, ksh and zsh (but not dash) supports any
	// number of arguments and loop through them; we follow their behavior.
	if opts.has('V') {
		// Identify the command. Almost identical to the type command, except
		// for the support of -p.
		ret := 0
		for _, arg := range args {
			status := identifyCommandType(fm, arg, path)
			if status != 0 {
				ret = status
			}
		}
		return ret
	} else if opts.has('v') {
		// Expand the command. Similar to -V with different output format.
		ret := 0
		for _, arg := range args {
			status := expandCommand(fm, arg, path)
			if status != 0 {
				ret = status
			}
		}
		return ret
	}
	// Execute the command. POSIX's specification requires at least one argument
	// (the command name), but all of dash, bash, ksh and zsh do nothing when
	// given no arguments.
	if len(args) == 0 {
		return 0
	}
	status, _ := fm.callCommand(args, fm.currentCommand, false)
	return status
}

func identifyCommandType(fm *frame, name, path string) int {
	var what string
	if isShellKeyword(name) {
		what = "a keyword"
	} else if def, ok := fm.aliases[name]; ok {
		what = "an alias for " + def
	} else if _, ok := specialBuiltins[name]; ok {
		what = "a special builtin"
	} else if _, ok := fm.functions[name]; ok {
		what = "a function"
	} else if _, ok := builtins[name]; ok {
		what = "a builtin"
	} else {
		path, status := fm.lookExecutable(name, path)
		if status == 0 {
			what = path
		} else {
			return status
		}
	}
	fmt.Fprintf(fm.files[1], "%v is %v\n", name, what)
	return 0
}

func expandCommand(fm *frame, name, path string) int {
	var what string
	if isShellKeyword(name) {
		what = name
	} else if def, ok := fm.aliases[name]; ok {
		what = def
	} else if _, ok := specialBuiltins[name]; ok {
		what = name
	} else if _, ok := fm.functions[name]; ok {
		what = name
	} else if _, ok := builtins[name]; ok {
		what = name
	} else {
		path, status := fm.lookExecutable(name, path)
		if status == 0 {
			what = path
		} else {
			return status
		}
	}
	fmt.Fprintln(fm.files[1], what)
	return 0
}

func isShellKeyword(name string) bool {
	switch name {
	case "for", "do", "done", "case", "esac", "if", "then", "elif", "else", "fi", "while", "until", "!", "{", "}":
		return true
	}
	return false
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
	ret := 0
	for _, arg := range args {
		status := identifyCommandType(fm, arg, fm.GetVar("PATH"))
		if status != 0 {
			ret = status
		}
	}
	return ret
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
	status := 0
	opts, args, err := getopts(args, "a")
	if err != nil {
		fm.badCommandLine("%v", err)
		return StatusBadCommandLine
	}
	for _, name := range args {
		if _, ok := fm.aliases[name]; ok {
			delete(fm.aliases, name)
		} else {
			// It would make more sense for this to not error for consistency
			// with unset, but this behavior is specified by POSIX.
			fmt.Fprintf(fm.files[2], "no alias definitions for %v", name)
			status = 1
		}
	}
	if opts.has('a') {
		clearMap(fm.aliases)
	}
	return status
}

func waitCmd(fm *frame, args []string) int {
	// TODO
	return 0
}
