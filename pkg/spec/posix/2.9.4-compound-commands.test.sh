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
