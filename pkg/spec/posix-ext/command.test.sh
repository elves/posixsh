# Same as type.test.sh.

#### command -V identifies command type
alias a=echo
f() { :; }
command -V if a eval f true
## STDOUT:
if is a keyword
a is an alias for echo
eval is a special builtin
f is a function
true is a builtin
## END

#### command -V shows full path of external commands
touch ls; chmod +x ls
PATH=$PWD
command -V ls
## stdout-regexp: ls is .+/ls\n

#### command -V errors if command is not found
PATH=$PWD
command -V ls
## status: 127
## stderr: command not found: ls

# TODO: Test for found but not executable; there doesn't seem to be a reliable
# way to create a file and ensure that it is not executable; the tmpfs may have
# quirks that make the files always executable.
