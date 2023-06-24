#### break inside a for loop
for x in foo bar; do
    echo $x
    if test $x = foo; then
        break
    fi
done
## stdout: foo
## status: 0

#### break inside a while loop
x=0
while true; do
    if test $x = 3; then
        break
    fi
    echo $x
    : $(( x += 1 ))
done
## STDOUT:
0
1
2
## END

#### break inside an until loop
x=0
until false; do
    if test $x = 3; then
        break
    fi
    echo $x
    : $(( x += 1 ))
done
## STDOUT:
0
1
2
## END

#### break aborting multiple levels
for x in foo bar; do
    echo $x
    y=0
    while true; do
        if test $y = 3; then
            break 2
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
## END

#### break raises fatal error when given invalid argument
break -1
echo should not get here
## status-interval: [1, 127]
## stdout:

#### break raises fatal error when given superfluous more arguments
break 1 10
echo should not get here
## status-interval: [1, 127]
## stdout:

#### break exits outermost loop when n > number of enclosing loops
for x in foo bar; do
    echo $x
    break 2
done
echo after
## STDOUT:
foo
after
## END

#### break stops where the shell execution environment changes
for x in foo bar; do
    echo $x
    (
        for y in lorem ipsum; do
            echo $y
            break 2
        done
    )
done
## STDOUT:
foo
lorem
bar
lorem
## END
