package eval

// Status codes returned by the shell itself.
//
// POSIX only specifies the status code for [CommandNotExecutable] and
// [CommandNotFound] and the status code when a command was killed by a signal.
// Errors during expansion or redirection are only required to have status codes
// between 1 and 125. See
// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_08_02.
//
// The practice of using 0 for no error is really well known, so we don't define
// a constant for it; code should just use 0.
const (
	// Same as dash and bash; zsh uses 1. Tested with: $sh -c 'if;'
	StatusSyntaxError = 2

	// Not sure what other shells use for the following error conditions.
	StatusPipeError = 100
	StatusWaitError = 101
	StatusWaitOther = 102

	// Specified by POSIX.
	StatusCommandNotExecutable = 126
	StatusCommandNotFound      = 127
	StatusSignalBase           = 128
)
