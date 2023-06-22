#### :
: foo
: ${x=foo}
echo $x
## status: 0
## stdout: foo

#### Official example 1
: ${X=abc}
if false
then :
else echo $X
fi
## stdout: abc

#### Official example 2
x=y : > z
echo $x
cat z
## stdout: y
## status: 0
