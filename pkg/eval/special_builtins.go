package eval

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/elves/posixsh/pkg/parse"
	"golang.org/x/sys/unix"
)

// Some builtins are designated "special" by POSIX; the return value includes a
// bool because they can return a fatal error that terminates evaluation.
//
// For more details on how special builtins differ from non-special builtins,
// see the code that uses this map.
var specialBuiltins = map[string]func(*frame, []string) (int, bool){
	"break":    breakCmd,
	":":        colonCmd,
	"continue": continueCmd,
	// "." and "eval" are set in init
	"exec":     execCmd,
	"exit":     exitCmd,
	"export":   exportCmd,
	"readonly": readonlyCmd,
	"return":   returnCmd,
	"set":      setCmd,
	"shift":    shiftCmd,
	"times":    timesCmd,
	"trap":     trapCmd,
	"unset":    unsetCmd,
}

func init() {
	// Some special builtins refer to methods that depend on specialBuiltins, so
	// initialize them here to avoid dependency cycle.
	specialBuiltins["."] = dotCmd
	specialBuiltins["eval"] = evalCmd
}

func breakCmd(fm *frame, args []string) (int, bool) {
	return breakOrContinue(fm, args, false)
}

// Implements break and continue. This works by returning (0, false) after
// setting fm.loopAbort, which is examined in the implementation of
// for/while/until.
func breakOrContinue(fm *frame, args []string, next bool) (int, bool) {
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

func colonCmd(*frame, []string) (int, bool) {
	return 0, true
}

func continueCmd(fm *frame, args []string) (int, bool) {
	return breakOrContinue(fm, args, true)
}

func dotCmd(fm *frame, args []string) (int, bool) {
	if len(args) == 0 {
		fm.badCommandLine(". requires at least one argument")
		return StatusBadCommandLine, false
	}
	path, ok, _ := fm.lookPath(args[0], 0)
	if !ok {
		fm.diagSpecialCommand("not found: %v\n", args[0])
		return StatusFileToSourceNotFound, false
	}
	bs, err := os.ReadFile(path)
	if err != nil {
		fm.diagSpecialCommand("cannot read %v: %v\n", args[0], err)
		return StatusFileToSourceNotReadable, false
	}
	code := string(bs)
	n, err := parse.Parse(code)
	if err != nil {
		fm.diagSpecialCommand("syntax error:", err)
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

func evalCmd(fm *frame, args []string) (int, bool) {
	code := strings.Join(args, " ")
	if strings.Trim(code, " \t\n") == "" {
		return 0, true
	}
	n, err := parse.Parse(code)
	if err != nil {
		fm.diagSpecialCommand("syntax error:", err)
		return StatusSyntaxError, false
	}
	return fm.chunk(n)
}

func execCmd(*frame, []string) (int, bool) {
	// TODO
	return 0, true
}

func exitCmd(fm *frame, args []string) (int, bool) {
	// POSIX doesn't specify the status should be when exit is called without an
	// argument, but all of dash, bash, ksh and zsh use $?; we follow this
	// behavior.
	status, ok := parseOneInt(fm, args, fm.lastPipelineStatus)
	if !ok {
		return StatusBadCommandLine, false
	}
	return status, false
}

func exportCmd(fm *frame, args []string) (int, bool) {
	return exportOrReadonly(fm, args, "export", fm.variables.exported)
}

func exportOrReadonly(fm *frame, args []string, cmd string, varSet set[string]) (int, bool) {
	// POSIX leaves the behavior of export and readonly unspecified when given
	// no argument, or when both -p and arguments are given. We follow the
	// behavior of dash here: parse the options, and print exported/readonly
	// variables if there are no arguments. This means that -p is actually an
	// no-op.
	_, args, ok := fm.getopt(args, "p")
	if !ok {
		return StatusBadCommandLine, false
	}
	if len(args) == 0 {
		// POSIX doesn't require the names to be sorted, but all of dash, bash,
		// ksh and zsh sort the names.
		names := sortedNames(varSet)
		for _, name := range names {
			value, set := fm.variables.values[name]
			if set {
				fmt.Fprintf(fm.files[1], "%v %v=%v\n", cmd, name, quote(value))
			} else {
				fmt.Fprintf(fm.files[1], "%v %v\n", cmd, name)
			}
		}
		return 0, true
	}
	for _, arg := range args {
		name, value, hasValue := strings.Cut(arg, "=")
		// Try to set the variable before adding it to the set. This order is
		// important in readonly; if we add it to the readonly set first, the
		// assignment will always fail.
		if hasValue {
			canSet := fm.SetVar(name, value)
			if !canSet {
				fm.diagSpecialCommand("%v is readonly\n", name)
				return StatusAssignmentError, false
			}
		}
		varSet.add(name)
	}
	return 0, true
}

func readonlyCmd(fm *frame, args []string) (int, bool) {
	return exportOrReadonly(fm, args, "readonly", fm.variables.readonly)
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

func setCmd(fm *frame, args []string) (int, bool) {
	if len(args) == 0 {
		// According to POSIX, a call to list variables must truly has no
		// argument, not even "--": "set --" is instead a call to unset all
		// positional parameters.
		printVariables(fm.files[1], fm.variables.values)
		return 0, true
	}
	// The options of set are different from other commands in two ways:
	//
	//  - Options may start with + (for turning off an option).
	//  - -o and +o take an optional argument *in the next argument*.
	//
	// POSIX only lists the usage of -o and +o as separate arguments, and the
	// forms that use them for listing options ("set -o" and "set +o") do not
	// mix with the form for setting options and positional parameters. We adopt
	// a similar strategy as bash: parse -o and +o like other options; they mean
	// "set option" when followed by another argument that is not "--", and mean
	// "list options" when followed by "--" or end of argument list.
	for len(args) > 0 {
		arg := args[0]
		if arg == "--" {
			args = args[1:]
			break
		} else if arg == "" {
			// Same as the else branch, but put it here so that we can look at
			// arg[0].
			break
		} else if on, off := arg[0] == '-', arg[0] == '+'; on || off {
			args = args[1:]
			for i := 1; i < len(arg); i++ {
				if arg[i] == 'o' {
					if len(args) > 0 && args[0] != "--" {
						// There is an argument after this that is not "--"; use
						// it as the option name.
						name := args[0]
						args = args[1:]
						if bit, ok := optionByName[name]; ok {
							fm.options = fm.options.with(bit, on)
						} else {
							fm.badCommandLine("unknown option %s", name)
						}
					} else {
						fmt.Fprint(fm.files[1], fm.options.format(off))
					}
				} else if bit, ok := optionByLetter[arg[i]]; ok {
					fm.options = fm.options.with(bit, on)
				} else {
					fm.badCommandLine("unknown option %s", arg[i])
					return StatusBadCommandLine, false
				}
			}
		} else {
			// Non-option argument; stop parsing. This is consistent with the
			// use of "BSD" style getopt for parsing options of other commands.
			break
		}
	}

	fm.arguments = append([]string{fm.arguments[0]}, args...)
	return 0, true
}

func printVariables(out *os.File, values map[string]string) {
	// POSIX requires that the names be sorted.
	names := sortedNames(values)
	for _, name := range names {
		fmt.Fprintf(out, "%v=%v\n", name, quote(values[name]))
	}
}

func shiftCmd(fm *frame, args []string) (int, bool) {
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

func timesCmd(fm *frame, args []string) (int, bool) {
	if len(args) != 0 {
		fm.badCommandLine("the times command accepts no arguments")
		return StatusBadCommandLine, false
	}
	// [unix.Times] is not defined for darwin; use the newer [unix.Getrusage]
	// instead.
	print := func(who int) {
		var rusage unix.Rusage
		err := unix.Getrusage(who, &rusage)
		if err != nil {
			fmt.Fprintln(fm.files[1], "?m?s ?m?s")
		} else {
			fmt.Fprintln(fm.files[1],
				formatTimevalForTimes(rusage.Utime), formatTimevalForTimes(rusage.Stime))
		}
	}
	print(unix.RUSAGE_SELF)
	print(unix.RUSAGE_CHILDREN)
	return 0, true
}

func trapCmd(*frame, []string) (int, bool) {
	// TODO
	return 0, true
}

func unsetCmd(fm *frame, args []string) (int, bool) {
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
		// This branch handles either explicit -v or no option. In both cases,
		// unset variables.
		for _, name := range args {
			delete(fm.variables.values, name)
		}
	}
	return 0, true
}

// Utilities used for implementing special builtins.

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

// The same as parse.normalBarewordStopper.
const nonBareword = "\r\n;)}&| \t<>(\\'\"$`"

func quote(s string) string {
	if !strings.ContainsAny(s, nonBareword) {
		return s
	}
	// Single-quote the value.
	s1 := strings.TrimLeft(s, "'")
	leadingQuotes := len(s) - len(s1)
	s2 := strings.TrimRight(s1, "'")
	trailingQuotes := len(s1) - len(s2)
	return strings.Repeat(`\'`, leadingQuotes) +
		"'" + strings.ReplaceAll(s2, "'", `'\''`) + "'" +
		strings.Repeat(`\'`, trailingQuotes)
}

func formatTimevalForTimes(t unix.Timeval) string {
	return fmt.Sprintf("%dm%fs",
		t.Sec/60, float64(t.Sec%60)+float64(t.Usec)/1e6)
}

func (fm *frame) diagSpecialCommand(format string, args ...any) {
	// TODO: Incorporate range information.
	fmt.Fprintf(fm.diagFile, format+"\n", args...)
}

func (fm *frame) badCommandLine(format string, args ...any) {
	// TODO: Incorporate range information.
	fmt.Fprintf(fm.diagFile, "bad command line option: "+format+"\n", args...)
}
