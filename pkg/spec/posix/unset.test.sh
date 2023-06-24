#### unset variables
x=foo
y=bar
echo ${x-lorem} ${y-ipsum}
unset x y
echo ${x-lorem} ${y-ipsum}
## STDOUT:
foo bar
lorem ipsum
## END

#### unset variables with explicit -v
x=foo
y=bar
echo ${x-lorem} ${y-ipsum}
unset -v x y
echo ${x-lorem} ${y-ipsum}
## STDOUT:
foo bar
lorem ipsum
## END

# TODO: Test unset functions with -f

# TODO: Test error when both -f and -v are given
