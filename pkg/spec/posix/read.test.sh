#### read
# Note: we can't use a pipeline of echo and read, since POSIX allows any part of
# a pipeline to be executed in a subshell.
echo 'foo bar' > file
read a b < file
printf ': %s\n' "$a" "$b"
## STDOUT:
: foo
: bar
## END

#### read respects IFS
IFS=:
echo 'foo bar:lorem' > file
read a b < file
printf ': %s\n' "$a" "$b"
## STDOUT:
: foo bar
: lorem
## END

#### read sets extra variables to empty when there are more variables than fields
echo 'foo bar' > file
read a b c < file
printf ': %s\n' "$a" "$b" "$c"
echo ${c+set}
## STDOUT:
: foo
: bar
: 
set
## END

#### read puts remainder in the last variable when there are more fields than variables
echo 'foo bar  lorem' > file
read a b < file
printf ': %s\n' "$a" "$b"
## STDOUT:
: foo
: bar  lorem
## END

#### read reads only one line
printf 'foo\nbar' > file
read a b < file
printf ': %s\n' "$a" "$b"
## STDOUT:
: foo
: 
## END

#### read interpretes backslashes, including line continuation
printf 'foo\\\nb\\a\\\\r' > file
PS2='more>'
read a < file
printf ': %s\n' $a
## stdout: : fooba\r
## stderr-json: "more>"
## END

#### read -r doesn't interprete backslashes
printf 'foo\\\nb\\a\\\\r' > file
read -r a < file
printf ': %s\n' $a
## STDOUT:
: foo\
## END
