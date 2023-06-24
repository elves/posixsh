#### continue does nothing when there is no loop
continue
echo after
## STDOUT:
after
## END

#### continue also aborts non-lexically enclosing loop
f() {
    for y in lorem ipsum; do
        echo $y
        continue 2
    done
}
for x in foo bar; do
    f
    echo $x
done
## STDOUT:
lorem
lorem
## END
