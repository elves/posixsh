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
