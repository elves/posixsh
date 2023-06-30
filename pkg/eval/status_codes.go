package eval

// Status codes returned by the shell itself.
//
// POSIX only specifies the status code for [StatusCommandNotExecutable] and
// [StatusCommandNotFound] and the status code when a command was killed by a
// signal. Errors during expansion or redirection are only required to have
// status codes between 1 and 125. See
// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_08_02.
//
// The practice of using 0 for no error is really well known, so we don't define
// a constant for it; code should just use 0.
const (
	// Same as dash and bash; ksh uses 3, zsh uses 1. Tested with:
	//
	//     $sh -c 'if;'
	StatusSyntaxError = 2
	// Same as dash; bash, ksh and zsh use 1. Tested with:
	//
	//     $sh -c 'echo $((1//2))'
	StatusExpansionError = 2
	// Same as dash; bash, ksh and zsh use 1. Tested with:
	//
	//     $sh -c 'cat < foo' # when foo doesn't exist
	StatusRedirectionError = 2
	// Same as dash and ksh; bash and zsh use 1. Tested with:
	//
	//     $sh -c 'unset -x'
	StatusBadCommandLine = 2
	// Same as dash and bash (when POSIXLY_CORRECT=1), which treat this error as
	// a syntax error. Ksh uses 1. Zsh doesn't consider this an error.
	//
	//     $sh -c 'break() { ; }'
	StatusInvalidFunctionName = 2

	// Same as dash; bash and ksh use 1, zsh uses 127 (same as command not
	// found). Tested with:
	//
	//     $sh -c '. foobar' # when foobar doesn't exist anywhere on PATH
	StatusFileToSourceNotFound = 2
	// Dash and zsh use 127 (same as command not found); bash and ksh use 1. We
	// used 2 for consistency with StatusFileToSourceNotFound. Tested with:
	//
	//     touch foobar; chmod -r foobar
	//     $sh -c '. ./foobar'
	StatusFileToSourceNotReadable = 2

	StatusRUsageError = 2

	StatusNotImplemented = 99

	// Relatively rare error conditions. Not sure what other shells use for
	// these.
	StatusPipeError = 100
	StatusWaitError = 101
	StatusWaitOther = 102
	StatusShellBug  = 103

	// Specified by POSIX.
	StatusCommandNotExecutable = 126
	StatusCommandNotFound      = 127
	StatusSignalBase           = 128
)
