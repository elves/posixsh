#### continue inside a for loop
for x in foo bar foobar; do
    if test $x = bar; then
        continue
    fi
    echo $x
done
## STDOUT:
foo
foobar
## END
## status: 0

#### continue inside a while loop
x=0
while test $x -lt 5; do
    : $(( x += 1 ))
    if test $x = 3; then
        continue
    fi
    echo $x
done
## STDOUT:
1
2
4
5
## END

#### continue inside an until loop
x=0
until test $x -ge 5; do
    : $(( x += 1 ))
    if test $x = 3; then
        continue
    fi
    echo $x
done
## STDOUT:
1
2
4
5
## END

#### continue aborting multiple levels
for x in foo bar; do
    echo $x
    y=0
    while true; do
        if test "$y" = 3; then
            continue 2
        fi
        echo $y
        : $(( y += 1 ))
    done
done
## STDOUT:
foo
0
1
2
bar
0
1
2
## END

#### continue raises fatal error when given invalid argument
continue -1
echo should not get here
## status: [1, 127]
## stdout:

#### continue raises fatal error when given superfluous more arguments
continue 1 10
echo should not get here
## status: [1, 127]
## stdout:

#### continue exits outermost loop when n > number of enclosing loops
for x in foo bar foobar; do
    if test $x = bar; then
        continue 2
    fi
    echo $x
done
echo after
## STDOUT:
foo
foobar
after
## END

#### continue stops where the shell execution environment changes
for x in foo bar; do
    echo $x
    (
        for y in lorem ipsum; do
            if test $y = lorem; then
                continue 2
            fi
            echo $y
        done
    )
done
## STDOUT:
foo
ipsum
bar
ipsum
## END
