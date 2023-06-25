#### Redirection error with a compound command causes it not to run, but is not fatal
( echo foo ) < bad-file
if true; then :; fi < bad-file
echo should get here
## stdout: should get here
## stderr-regexp: .+

#### Redirection error with a function causes it not to run, but is not fatal
f() { echo in f; }
f < bad-file
echo should get here
## stdout: should get here
## stderr-regexp: .+

#### Command not found is not fatal
PATH=
nonexistent-command
echo should get here
## stdout: should get here
## stderr-regexp: .+
