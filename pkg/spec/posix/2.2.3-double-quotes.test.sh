#### Double quotes: variable
foo=bar
echo "foo=$foo"
echo "foo=${foo}"
## STDOUT:
foo=bar
foo=bar
## END

#### Double quotes: command subtitution with $
echo "output=$(echo "foo")"
## STDOUT:
output=foo
## END

#### Double quotes: arithmetic expansion
echo "answer=$(( 7 * 6 ))"
## STDOUT:
answer=42
## END

#### Double quotes: command substitution with `
echo "output=`echo foo`"
## STDOUT:
output=foo
## END
