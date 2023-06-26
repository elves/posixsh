#### Pipeline
printf 'foo\nbar\n' | sed s/o/0/g | sed s/r/R/g
## STDOUT:
f00
baR
## END

#### Status of pipeline is that of the last command
true | false
echo $?
false | true
echo $?
## STDOUT:
1
0
## END

#### Status of pipeline prefixed with "!" is negated
! true | false
echo $?
! false | true
echo $?
## STDOUT:
0
1
## END
