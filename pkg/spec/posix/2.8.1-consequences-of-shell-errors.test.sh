#### Syntax error is fatal
if; fi
echo should not get here
## status: [1, 127]
## stdout-json: ""
## stderr-regexp: .+

#### Error from special builtin is fatal
break 1 2 3 # break is a special builtin
echo should not get here
## status: [1, 127]
## stdout-json: ""
## stderr-regexp: .+

# TODO:
# #### Error from other utilities are not fatal

#### Redirection error with special builtin is fatal
break < bad-file
echo should not get here
## status: [1, 127]
## stdout-json: ""
## stderr-regexp: .+

#### Redirection error with other utilities are not fatal
true < bad-file
echo should get here
## stdout: should get here
## stderr-regexp: .+

#### Variable assignment error is fatal
readonly x=foo
x=bar
echo should not get here
## status: [1, 127]
## stdout-json: ""
## stderr-regexp: .+

#### Variable assignment error in arithmetic expression is fatal
readonly x=foo
: $(( x++ ))
echo should not get here
## status: [1, 127]
## stdout-json: ""
## stderr-regexp: .+

#### Variable assignment error in ${name=value} expression is fatal
readonly x
: ${x=value}
echo should not get here
## status: [1, 127]
## stdout-json: ""
## stderr-regexp: .+

#### Variable assignment error in for command is fatal
readonly x
for x in a b; do
    :
done
echo should not get here
## status: [1, 127]
## stdout-json: ""
## stderr-regexp: .+

#### Expansion error is fatal
echo $(( 1 /*/ 2 ))
echo should not get here
## status: [1, 127]
## stdout-json: ""
## stderr-regexp: .+

#### Fatal errors in subshell only exit the subshell
( echo $(( 1 /*/ 2 )) )
echo should get here
## stdout: should get here
## stderr-regexp: .+
