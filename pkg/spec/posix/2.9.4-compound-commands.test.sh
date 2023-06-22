#### ( ) group
x=old
( echo output; x=new; echo $x )
echo $x
## STDOUT:
output
new
old
## END

# TODO: Also test that cd inside ( ) doesn't affect outside environment

#### Status of ( ) group
( false )
## status: 1

# TODO: More status tests with $?

#### { } group
x=old
{ echo output; x=new; echo $x; }
echo $x
## STDOUT:
output
new
new
## END

#### Status of { } group
{ false; }
## status: 1

# TODO: More status tests with $?

#### for
for x in foo bar; do
    echo $x
done
## STDOUT:
foo
bar
## END

#### for without "in word..." defaulting to positional parameters
set -- arg1 arg2
for x; do
    echo $x
done
## STDOUT:
arg1
arg2
## END

#### Status of for
for cmd in true false; do
    $cmd
done
## status: 1

# TODO: More status tests with $?

#### case
x=foo
case $x in
bar) echo is bar
     echo more
     ;;
foo) echo is foo ;;
esac
## stdout: is foo

#### case with ( in pattern
x=foo
case $x in
(bar) echo is bar ;;
(foo) echo is foo ;;
esac
## stdout: is foo

#### case with terminating ;; omitted
x=foo
case $x in
(bar) echo is bar ;;
(foo) echo is foo
esac
## stdout: is foo

#### case with multiple choices in a branch
x=foo
case $x in
bar) echo is bar ;;
baz|foo) echo is baz or foo ;;
esac
## stdout: is baz or foo

#### case with pattern matching
x=foo
case $x in
o*) echo starts with o ;;
[fx]*) echo starts with f or x
esac
## stdout: starts with f or x

#### Status of case
x=foo
case $x in
bar) true ;;
foo) false ;;
esac
# status: 1

# TODO: More status tests with $?

#### if
if true; then
    echo true
fi
if false; then
    echo false
fi
## stdout: true

#### if with else
if true; then
    echo true
else
    echo true-else
fi
if false; then
    echo false
else
    echo false-else
fi
## STDOUT:
true
false-else
## END

#### if with elif
if false; then
    echo false
elif true; then
    echo false-elif-true
fi
## stdout: false-elif-true

# TODO: More cases?

#### Status of if
if true; then
    false
fi
## status: 1

#### Status of if when no branch is executed
if false; then
    false
fi
## status: 0

# TODO: More status tests?

#### while
x=0
while test $x -lt 4; do
    echo $x
    : $(( x += 1 ))
done
## STDOUT:
0
1
2
3
## END

#### Status of while
x=0
while test $x -lt 4; do
    : $(( x += 1 ))
    false
done
## status: 1

#### Status of while when no loop is executed
x=0
while test $x -lt -1; do
    false
done
## status: 0

#### until
x=0
until test $x -ge 4; do
    echo $x
    : $(( x += 1 ))
done
## STDOUT:
0
1
2
3
## END

#### Status of while
x=0
until test $x -ge 4; do
    : $(( x += 1 ))
    false
done
## status: 1

#### Status of while when no loop is executed
x=0
until test $x -ge -1; do
    false
done
## status: 0

# TODO: Test more complex conditions in if, while, until

# TODO: Test redirections in non-simple commands

# TODO: Test function definition
