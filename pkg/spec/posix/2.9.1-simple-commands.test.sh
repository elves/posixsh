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

#### Assignments when command is a non-special builtin only affect that command
x=old
x=new true # true is a non-special builtin
echo $x
## stdout: old

#### Assignments when command is an external command only affect that command
x=old
export x
x=new sh -c 'echo $x'
echo $x
## STDOUT:
new
old
## END

#### Assignments when command is a special builtin affects the current environment
x=old
x=new :
echo $x
## stdout: new

#### Assignments when command is a function affects that command
x=old
f() {
    echo $x
}
x=new f
# Whether x is old or new at this point is unspecified by POSIX, so we don't
# test it here.
## stdout: new

# TODO: Test consequence of assigning to a readonly variable

#### Redirection when there is no command name doesn't affect the current execution environment
> some-file
echo foo
## stdout: foo

#### Redirection error causes an immediate failure but is not fatal
echo should not appear < non-existent
echo should get here
## stdout: should get here
## stderr-regexp: .+

#### Status is that of the last command substitution if there is no command name
x=$(true) y=$(false)
## status: 1

#### Status is 0 if there is no command substitution and no command name
x=foo y=bar
## status: 0

#### Command search order: special builtins before external commands
# Note: Search for special builtins also takes place before functions and
# non-special builtins, but since the two classes can't have names that are the
# same as special commands, this effect is not actually observable.
printf '#!/bin/sh\necho script break' > break
chmod +x break
PATH=$PWD:$PATH
break
## stdout-json: ""

#### Command search order: functions before non-special builtins
true() {
  echo function true
}
true
## stdout: function true

#### Command search order: non-special builtins before external commands
printf '#!/bin/sh\necho script true' > true
chmod +x true
PATH=$PWD:$PATH
true
## stdout-json: ""

#### Execution of external commands
printf '#!/bin/sh\necho script foo $1 $2' > foo
chmod +x foo
PATH=$PWD:$PATH
foo lorem ipsum
## stdout: script foo lorem ipsum

#### Execution of external commands handles ENOEXEC by invoking shell
printf 'echo script foo $1 $2' > foo
chmod +x foo
PATH=$PWD:$PATH
foo lorem ipsum

#### Command search doesn't take place if command name contains slash
mkdir d
printf '#!/bin/sh\necho script foo $1 $2' > foo
printf '#!/bin/sh\necho script d/foo $1 $2' > d/foo
chmod +x foo d/foo
./foo 1 2
d/foo 3 4
## STDOUT:
script foo 1 2
script d/foo 3 4
## END
