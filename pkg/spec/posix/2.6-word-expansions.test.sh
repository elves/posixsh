# TODO: Test the order of word expansions.

#### Words expanding to a single null field are deleted
x=''
printf ': %s\n' $x $(echo) $x$(echo) foo
## STDOUT:
: foo
## END

#### Single-quoted or partially single-quoted empty words are not deleted
x=''
printf ': %s\n' '' ''$x $x'' $(echo)'' ''$(echo) foo
## STDOUT:
: 
: 
: 
: 
: 
: foo
## END

#### Double-quoted or partially double-quoted empty words are not deleted
x=''
printf ': %s\n' "" ""$x $x"" ""$(echo) $(echo)"" foo
## STDOUT:
: 
: 
: 
: 
: 
: foo
## END
