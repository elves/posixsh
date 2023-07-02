#### type identifies command type
alias a=echo
f() { :; }
type if a eval f true
## STDOUT:
if is a keyword
a is an alias for echo
eval is a special builtin
f is a function
true is a builtin
## END

#### type shows full path of external commands
touch ls; chmod +x ls
PATH=$PWD
type ls
## stdout-regexp: ls is .+/ls\n

#### type errors if command is not found
PATH=$PWD
type ls
## status: 127
## stderr: command not found: ls

# TODO: Test for found but not executable; there doesn't seem to be a reliable
# way to create a file and ensure that it is not executable; the tmpfs may have
# quirks that make the files always executable.
