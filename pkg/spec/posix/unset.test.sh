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

#### unset functions with -f
true() {
    echo new true
}
false() {
    echo new false
    return 1
}
true
false
echo after unset
unset -f true false
true
false
## status: [1, 127]
## STDOUT:
new true
new false
after unset
## END
