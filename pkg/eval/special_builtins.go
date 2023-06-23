package eval

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
	"eval":     eval,
	"export":   export,
	"readonly": readonly,
	"return":   returnCmd,
	"set":      set,
	"shift":    shift,
	"times":    times,
	"trap":     trap,
	"unset":    unset,
}

func breakCmd(*frame, []string) (int, bool) {
	// TODO
	return 0, true
}

func colon(*frame, []string) (int, bool) {
	return 0, true
}

func continueCmd(*frame, []string) (int, bool) {
	// TODO
	return 0, true
}

func dot(*frame, []string) (int, bool) {
	// TODO
	return 0, true
}

func eval(*frame, []string) (int, bool) {
	// TODO
	return 0, true
}

func export(*frame, []string) (int, bool) {
	// TODO
	return 0, true
}

func readonly(*frame, []string) (int, bool) {
	// TODO
	return 0, true
}

func returnCmd(*frame, []string) (int, bool) {
	// TODO
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
		return StatusCommandLineError, false
	}
	if opts.isSet('f') && opts.isSet('v') {
		fm.badCommandLine("-f and -v are mutually exclusive")
		return StatusCommandLineError, false
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
