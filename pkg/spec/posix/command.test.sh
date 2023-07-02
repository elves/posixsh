#### command -v writes keywords, special builtins, functions and builtins as is
f() { :; }
command -v if
command -v eval
command -v f
command -v true
## STDOUT:
if
eval
f
true
## END

#### command -v writes definition of aliases
alias a=echo
command -v a
## stdout: echo

#### command -v writes full path of external commands
touch ls; chmod +x ls
PATH=$PWD
command -v ls
## stdout-regexp: .+/ls\n

#### command executes special builtin
command eval 'echo command'
## stdout: command

#### command turns fatal errors from special builtin to non-fatal errors
command eval 'echo $(( 1 /*/ 2 ))' ||
echo should get here
## stdout: should get here
## stderr-regexp: .+

#### command executes builtin
command true &&
echo should get here
## stdout: should get here

#### command executes external commands
printf '#!/bin/sh\necho external' > script
chmod +x script
PATH=$PWD:$PATH
command script
## stdout: external

#### command doesn't execute alias
alias echo='echo foo'
command echo bar
## stdout: bar

#### command doesn't execute function
f() { :; }
PATH=
command f
## status: 127
## stderr-regexp: .+

# TODO: Test -p

# The format of command -V is unspecified.

#### command -V succeeds when given valid command name
alias a=echo
f() { :; }
command -V if &&
command -V a &&
command -V eval &&
command -V f &&
command -V true &&
command -V ls
## status: 0
## stdout-regexp: .+

#### command -V returns 127 on command not found
PATH=
type bad-command
## status: 127
## stderr-regexp: .+
