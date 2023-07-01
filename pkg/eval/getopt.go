package eval

import (
	"src.elv.sh/pkg/getopt"
)

type parsedOpt struct {
	name  byte
	value string
}

type parsedOpts []parsedOpt

func (p parsedOpts) get(name byte) (string, bool) {
	for _, o := range p {
		if o.name == name {
			return o.value, true
		}
	}
	return "", false
}

func (p parsedOpts) has(name byte) bool {
	_, exists := p.get(name)
	return exists
}

// A wrapper around [getopt.Parse], optimized for use cases that only need short
// options, which is the case for all the shell builtins standardized by POSIX.
// The API mimics the C function with the same name.
func getopts(args []string, optstring string) (parsedOpts, []string, error) {
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
		return nil, nil, err
	}
	parsed := make(parsedOpts, len(opts))
	for i, opt := range opts {
		parsed[i] = parsedOpt{byte(opt.Spec.Short), opt.Argument}
	}
	return parsed, args, nil
}
