# The output format of the type command is unspecified in POSIX.

#### type succeeds when given valid command name
alias a=echo
f() { :; }
type if a eval f true ls
## status: 0
## stdout-regexp: .+

#### type returns 127 on command not found
PATH=
type bad-command
## status: 127
## stderr-regexp: .+
