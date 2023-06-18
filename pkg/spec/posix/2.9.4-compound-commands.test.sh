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

# TODO: Test pattern matching

#### Status of case
x=foo
case $x in
bar) true ;;
foo) false ;;
esac
# status: 1

# TODO: More status tests with $?
