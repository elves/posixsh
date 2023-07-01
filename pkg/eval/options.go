package eval

import (
	"fmt"
	"strings"
)

type options uint32

// Omitted: ignoreeof, nolog, vi. These are all related to interactive mode and
// unlikely to be found in scripts.
const (
	allexport options = 1 << iota
	errexit
	monitor
	noclobber
	noglob
	noexec
	notify
	nounset
	verbose
	xtrace
)

// Omitted: -h. Dash doesn't have this option, and bash and zsh use -h for
// something else, so it's unlikely that any script will use it.
var optionByLetter = map[byte]options{
	'a': allexport,
	'e': errexit,
	'm': monitor,
	'C': noclobber,
	'f': noglob,
	'n': noexec,
	'b': notify,
	'u': nounset,
	'v': verbose,
	'x': xtrace,
}

var optionByName = map[string]options{
	"allexport": allexport,
	"errexit":   errexit,
	"monitor":   monitor,
	"noclobber": noclobber,
	"noglob":    noglob,
	"noexec":    noexec,
	"notify":    notify,
	"nounset":   nounset,
	"verbose":   verbose,
	"xtrace":    xtrace,
}

func (o options) has(bit options) bool {
	return o&bit != 0
}

func (o options) with(bit options, on bool) options {
	if on {
		return o | bit
	} else {
		return o &^ bit
	}
}

// Use for printing options with "set -o" or "set +o". POSIX specifies that "set
// +o" should print commands that can be used to recreate the options, but
// leaves the format of "set -o" unspecified. All of bash, dash, ksh and zsh use
// a tabular format with long names and "on/off", so we follow their behavior.
func (o options) format(asCommands bool) string {
	var sb strings.Builder
	names := sortedNames(optionByName)
	for _, name := range names {
		set := o.has(optionByName[name])
		var format string
		if asCommands {
			if set {
				format = "set -o %v\n"
			} else {
				format = "set +o %v\n"
			}
		} else {
			if set {
				format = "%-10v on\n"
			} else {
				format = "%-10v off\n"
			}
		}
		fmt.Fprintf(&sb, format, name)
	}
	return sb.String()
}
