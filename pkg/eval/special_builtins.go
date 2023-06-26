package eval

import (
	"fmt"
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
	".":        dot,
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
	specialBuiltins["eval"] = eval
}

func breakCmd(fm *frame, args []string) (int, bool) {
	return abortLoop(fm, args, false)
}

// Implements break and continue. This works by returning (0, false) after
// setting fm.loopAbort, which is examined in the implementation of
// for/while/until.
func abortLoop(fm *frame, args []string, next bool) (int, bool) {
	var level int
	switch len(args) {
	case 0:
		level = 1
	case 1:
		n, err := strconv.Atoi(args[0])
		if err != nil {
			fm.badCommandLine("argument must be number, got %q", args[0])
			return StatusBadCommandLine, false
		}
		if n <= 0 {
			fm.badCommandLine("argument must be positive, got %v", n)
			return StatusBadCommandLine, false
		}
		level = n
	default:
		fm.badCommandLine("at most 1 argument accepted, got %v", len(args))
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

func colon(*frame, []string) (int, bool) {
	return 0, true
}

func continueCmd(fm *frame, args []string) (int, bool) {
	return abortLoop(fm, args, true)
}

func dot(*frame, []string) (int, bool) {
	// TODO
	return 0, true
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

func export(*frame, []string) (int, bool) {
	// TODO
	return 0, true
}

func readonly(*frame, []string) (int, bool) {
	// TODO
	return 0, true
}

func returnCmd(fm *frame, args []string) (int, bool) {
	var status int
	switch len(args) {
	case 0:
		status = fm.lastPipelineStatus
	case 1:
		n, err := strconv.Atoi(args[0])
		if err != nil {
			fm.badCommandLine("argument must be number, got %q", args[0])
			return StatusBadCommandLine, false
		}
		if n < 0 {
			fm.badCommandLine("argument must be non-negative, got %v", n)
			return StatusBadCommandLine, false
		}
		status = n
	default:
		fm.badCommandLine("at most 1 argument accepted, got %v", len(args))
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

func shift(*frame, []string) (int, bool) {
	// TODO
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
