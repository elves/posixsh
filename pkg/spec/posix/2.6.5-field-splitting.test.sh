#### Parameter expansion
x='foo bar'
printf ': %s\n' $x
## STDOUT:
: foo
: bar
## END

#### Command substitution
printf ': %s\n' $(echo foo bar)
## STDOUT:
: foo
: bar
## END

#### Arithmetic expansion
IFS=0
printf ': %s\n' $(( 99 + 2 ))
## STDOUT:
: 1
: 1
## END

#### No field splitting for escaped space
printf ': %s\n' foo\ bar
## STDOUT:
: foo bar
## END

#### No field splitting for single-quoted strings
printf ': %s\n' 'foo bar'
## STDOUT:
: foo bar
## END

#### No field splitting inside double quotes
printf ': %s\n' "foo bar $(printf ' lorem\nipsum')"
## STDOUT:
: foo bar  lorem
ipsum
## END

#### No field splitting for double-quoted strings
nlx=$(printf '\nx')
printf ': %s\n' "foo bar$nlx"
## STDOUT:
: foo bar
x
## END

# Note: The special $@ is tested in 2.5.2-special-parameters.test.sh.

#### Default IFS
# Ignore leading and trailing [ \t\n]*, and use [ \t\n]+ to split fields.
x=$(printf ' \t\n foo  \t\n   bar \n\t  ')
printf ': %s\n' $x
## STDOUT:
: foo
: bar
## END

#### IFS with both whitespace and non-whitespace
IFS=' :/'
# IFS contains one whitespace (space) and two non-whitespaces (: and /). Ignore
# leading and trailing spaces and use "[ ]*[:/][ ]*|[ ]+" to split fields.
x=$(printf ' \na: b // c d')
printf ': %s\n' $x
## STDOUT:
: 
a
: b
: 
: c
: d
## END

#### IFS with only whitespaces
IFS=' '
# IFS contains only one whitespace (space). Ignore leading and trailing spaces
# and use "[ ]+" to split fields.
x=$(printf ' \na   b')
printf ': %s\n' $x
## STDOUT:
: 
a
: b
## END

#### IFS with only non-whitespaces
IFS=':/'
# IFS contains only two non-whitespaces. Preserve leading and trailing
# whitespaces, and use "[:/]" to split fields.
x=$(printf '\n a b:c// d ')
printf ': %s\n' $x
## STDOUT:
: 
 a b
: c
: 
:  d 
## END
