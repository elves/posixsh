#### Order of evaluation
# Arguments > redirections > assignments
x=old-x
x=$(echo value; echo assign1 >&2) \
  y=$(echo assign2 >&2) \
  echo \
  > $(echo file; echo redir1 >&2) \
  3> $(echo file3; echo redir2 >&2) \
  $(echo value; echo arg1 >&2) \
  $(echo arg2 >&2) \
  $x
cat file
## STDOUT:
value old-x
## END
## STDERR:
arg1
arg2
redir1
redir2
assign1
assign2
## END

#### Assignments with no command name affect the current environment
x=new
echo $x
## stdout: new

# TODO:
# #### Assignments when command is not a special builtin or function only affects that command

# TODO:
# #### Assignments when command is a special builtin affects the current environment

#### Assignments when command is a function affects that command
x=old
f() {
    echo $x
}
x=new f
# Whether x is old or new at this point is unspecified by POSIX
## stdout: new

# TODO: Test consequence of assigning to a readonly variable

# TODO: POSIX specifies that "if there is no command name, any redirections
# shall be performed in a subshell environment" - figure out what the
# consequence of subshell environment is

# TODO: Test redirection failure when there's support for asserting status != 0

#### Status is that of the last command substitution if there is no command name
x=$(true) y=$(false)
## status: 1

#### Status is 0 if there is no command substitution and no command name
x=foo y=bar
## status: 0

# TODO: Test the "command search and execution" subsection
