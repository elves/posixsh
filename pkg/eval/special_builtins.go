package eval

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/elves/posixsh/pkg/parse"
)

// Some builtins are designated "special" by POSIX; the return value includes a
// bool because they can return a fatal error that terminates evaluation.
//
// For more details on how special builtins differ from non-special builtins,
// see the code that uses this map.
var specialBuiltins = map[string]func(*frame, []string) (int, bool){
	"break":    breakCmd,
	":":        colon,
	"continue": continueCmd,
	// "." and "eval" are set in init
	"exec":     exec,
	"exit":     exit,
	"export":   export,
	"readonly": readonly,
	"return":   returnCmd,
	"set":      set,
	"shift":    shift,
	"times":    times,
	"trap":     trap,
	"unset":    unset,
}

func init() {
	// Some special builtins refer to methods that depend on specialBuiltins, so
	// initialize them here to avoid dependency cycle.
	specialBuiltins["."] = dot
	specialBuiltins["eval"] = eval
}

func breakCmd(fm *frame, args []string) (int, bool) {
	return abortLoop(fm, args, false)
}

func colon(*frame, []string) (int, bool) {
	return 0, true
}

func continueCmd(fm *frame, args []string) (int, bool) {
	return abortLoop(fm, args, true)
}

func dot(fm *frame, args []string) (int, bool) {
	if len(args) == 0 {
		fm.badCommandLine(". requires at least one argument")
		return StatusBadCommandLine, false
	}
	path, ok, _ := fm.lookPath(args[0], 0)
	if !ok {
		// TODO: Add range information.
		fmt.Fprintf(fm.diagFile, "not found: %v\n", args[0])
		return StatusFileToSourceNotFound, false
	}
	bs, err := os.ReadFile(path)
	if err != nil {
		// TODO: Add range information.
		fmt.Fprintf(fm.diagFile, "cannot read %v: %v\n", args[0], err)
		return StatusFileToSourceNotReadable, false
	}
	code := string(bs)
	n, err := parse.Parse(code)
	if err != nil {
		// TODO: Add range information.
		fmt.Fprintln(fm.diagFile, "syntax error:", err)
		return StatusSyntaxError, false
	}
	// A file sourced by "." can use the return command, like in a function
	// call. All of bash, ksh and zsh (but not dash) extend this similarity
	// further by setting the positional arguments within the execution. This is
	// not specified by POSIX, but we do that too.
	return fm.callFuncLike(args[1:], func() (int, bool) {
		return fm.chunk(n)
	})
}

func eval(fm *frame, args []string) (int, bool) {
	code := strings.Join(args, " ")
	if strings.Trim(code, " \t\n") == "" {
		return 0, true
	}
	n, err := parse.Parse(code)
	if err != nil {
		// TODO: Add range information.
		fmt.Fprintln(fm.diagFile, "syntax error:", err)
		return StatusSyntaxError, false
	}
	return fm.chunk(n)
}

func exec(*frame, []string) (int, bool) {
	// TODO
	return 0, true
}

func exit(fm *frame, args []string) (int, bool) {
	// POSIX doesn't specify the status should be when exit is called without an
	// argument, but all of dash, bash, ksh and zsh use $?; we follow this
	// behavior.
	status, ok := parseOneInt(fm, args, fm.lastPipelineStatus)
	if !ok {
		return StatusBadCommandLine, false
	}
	return status, false
}

func export(*frame, []string) (int, bool) {
	// TODO
	return 0, true
}

func readonly(*frame, []string) (int, bool) {
	// TODO
	return 0, true
}

func returnCmd(fm *frame, args []string) (int, bool) {
	status, ok := parseOneInt(fm, args, fm.lastPipelineStatus)
	if !ok {
		return StatusBadCommandLine, false
	}
	if status < 0 {
		fm.badCommandLine("argument must be non-negative, got %v", status)
		return StatusBadCommandLine, false
	}
	fm.fnAbort = true
	return status, false
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

func shift(fm *frame, args []string) (int, bool) {
	n, ok := parseOneInt(fm, args, 1)
	if !ok {
		return StatusBadCommandLine, false
	}
	if n > len(fm.arguments)-1 {
		fm.badCommandLine("argument to shift must not be larger than $#")
		return StatusBadCommandLine, false
	}
	copy(fm.arguments[1:], fm.arguments[1+n:])
	fm.arguments = fm.arguments[:len(fm.arguments)-n]
	return 0, true
}

func times(*frame, []string) (int, bool) {
	// TODO
	return 0, true
}

func trap(*frame, []string) (int, bool) {
	// TODO
	return 0, true
}

func unset(fm *frame, args []string) (int, bool) {
	opts, args, ok := fm.getopt(args, "fv")
	if !ok {
		return StatusBadCommandLine, false
	}
	if opts.isSet('f') && opts.isSet('v') {
		fm.badCommandLine("-f and -v are mutually exclusive")
		return StatusBadCommandLine, false
	}
	if opts.isSet('f') {
		for _, name := range args {
			delete(fm.functions, name)
		}
	} else {
		// When neither -f and -v is specified, default to variable
		for _, name := range args {
			delete(fm.variables, name)
		}
	}
	return 0, true
}

// Utilities used for implementing special builtins.

// Implements break and continue. This works by returning (0, false) after
// setting fm.loopAbort, which is examined in the implementation of
// for/while/until.
func abortLoop(fm *frame, args []string, next bool) (int, bool) {
	level, ok := parseOneInt(fm, args, 1)
	if !ok {
		return StatusBadCommandLine, false
	}
	if level <= 0 {
		fm.badCommandLine("argument must be positive, got %v", level)
		return StatusBadCommandLine, false
	}
	if fm.loopDepth == 0 {
		// POSIX leaves the behavior of break and continue unspecified when
		// there is no enclosing loop. We just let it do nothing; this is
		// consistent with the behavior when n is greater than the number of
		// enclosing loops.
		//
		// This behavior is shared by dash and ksh, but not by bash and zsh.
		return 0, true
	}
	dest := fm.loopDepth - level
	// POSIX specifies that break and continue should work on the outermost
	// loop when n > number of enclosing loops.
	if dest < 0 {
		dest = 0
	}
	fm.loopAbort = &loopAbort{dest: dest, next: next}
	return 0, false
}

func parseOneInt(fm *frame, args []string, fallback int) (int, bool) {
	switch len(args) {
	case 0:
		return fallback, true
	case 1:
		n, err := strconv.Atoi(args[0])
		if err != nil {
			fm.badCommandLine("argument must be number, got %q", args[0])
			return 0, false
		}
		return n, true
	default:
		fm.badCommandLine("at most 1 argument accepted, got %v", len(args))
		return 0, false
	}
}
