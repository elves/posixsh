# TODO: Test the order of word expansions.

#### Null words are deleted
x=''
printf ': %s\n' $x $(echo) $x$(echo) foo
## STDOUT:
: foo
## END

#### Null words are deleted even when IFS is empty
IFS=''
x=''
printf ': %s\n' $x $(echo) $x$(echo) foo
## STDOUT:
: foo
## END

#### Non-null words expanding to a single null word is deleted
x='   '
printf ': %s\n' $x foo
## STDOUT
: foo
## END

#### Single-quoted or partially single-quoted null words are not deleted
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

#### Double-quoted or partially double-quoted null words are not deleted
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
