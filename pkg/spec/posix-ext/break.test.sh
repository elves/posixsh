#### break does nothing when there is no loop
break
echo after
## STDOUT:
after
## END

#### break also aborts non-lexically enclosing loop
f() {
    for y in lorem ipsum; do
        echo $y
        break 2
    done
}
for x in foo bar; do
    echo $x
    f
done
## STDOUT:
foo
lorem
## END
