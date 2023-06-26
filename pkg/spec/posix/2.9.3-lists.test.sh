# TODO:
# #### Asynchronous list

# TODO:
# #### Status of asynchronous list is 0

#### Sequential list
echo foo
echo bar
## STDOUT:
foo
bar
## END

#### Sequential list separated by semicolons
echo foo; echo bar
## STDOUT:
foo
bar
## END

#### Status of sequential list is that of the last executed command
true
false
echo $?

false
true
echo $?
## STDOUT:
1
0
## END

#### a && b runs b if a succeeds
true && echo true
## stdout: true

#### a && b skips b if a fails
false && echo true
## stdout-json: ""
## status: 1

#### Status of a && b is that of the last executed command
true && false
echo $?
true && true
echo $?
false && true
echo $?
false && false
echo $?
## STDOUT:
1
0
1
1
## END

#### a || b skips b if a succeeds
true || echo true
## stdout-json: ""

#### a || b runs b if a fails
false || echo false
## stdout: false

#### Status of a || b is that of the last executed command
true || false
echo $?
true || true
echo $?
false || true
echo $?
false || false
echo $?
## STDOUT:
0
0
0
1
## END

#### Mix of && and || runs from left to right
#                                   If && and || have the same priority
#                                           If && has higher priority
#                                                       If || has higher priority
true || echo foo && echo bar      # bar   | (nothing) | bar
false && echo lorem || echo ipsum # ipsum | ipsum     | (nothing)
## STDOUT:
bar
ipsum
## END
