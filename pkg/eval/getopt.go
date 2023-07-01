package eval

import (
	"src.elv.sh/pkg/getopt"
)

type parsedOpts map[byte]string

func (p parsedOpts) isSet(b byte) bool {
	_, ok := p[b]
	return ok
}

// A wrapper around [getopt.Parse], optimized for use cases that only need short
// options, which is the case for all the shell builtins standardized by POSIX.
// The API mimics the C function with the same name.
func (fm *frame) getopt(args []string, optstring string) (parsedOpts, []string, bool) {
	var specs []*getopt.OptionSpec
	for i := 0; i < len(optstring); i++ {
		spec := &getopt.OptionSpec{Short: rune(optstring[i])}
		if i+1 < len(optstring) && optstring[i+1] == ':' {
			spec.Arity = getopt.RequiredArgument
			i++
		}
		specs = append(specs, spec)
	}
	// GNU style allows options to be mixed with operands, like "ls a -l b",
	// whereas BSD style doesn't. POSIX says all options "should" precede
	// options (12.2 Utility Syntax Guidelines, Guideline 9) - in other words,
	// BSD style - but it's not a hard requirement (which would use "shall"
	// instead).
	//
	// All of dash, ksh and zsh use the BSD style, while bash uses the GNU
	// style. We follow the advice of POSIX as well as the majority.
	opts, args, err := getopt.Parse(args, specs, getopt.BSD)
	if err != nil {
		fm.badCommandLine("%v", err)
		return parsedOpts{}, nil, false
	}
	parsed := make(parsedOpts, len(opts))
	for _, opt := range opts {
		parsed[byte(opt.Spec.Short)] = opt.Argument
	}
	return parsed, args, true
}
