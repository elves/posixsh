#### eval runs code
eval 'echo foo'
## stdout: foo

#### eval concatenates arguments with spaces
eval echo \' foo \' bar
## STDOUT:
 foo  bar
## END

#### eval throws a fatal error on syntax error
eval '$'
echo should not get here
## status: [1, 127]
## stdout:

#### Status of eval is the status of the last command
eval 'false'
## status: 1
