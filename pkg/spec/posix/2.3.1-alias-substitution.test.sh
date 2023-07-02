# This implementation only supports a limited version of alias substitution.

#### Alias substitution
alias hello='echo hello'
hello world
## stdout: hello world

#### Aliases can shadow special builtins
alias true=false
true
echo $?
## stdout: 1

#### Alias substitution is recursive
alias hello='echo hello'
alias world='hello world'
world peace
## stdout: hello world peace

#### Alias substitution doesn't expand the same alias more than once
alias echo='echo2 foo'
alias echo2='echo bar'
echo lorem # echo -> echo2 foo -> echo bar foo (stop since seeing echo again)
## stdout: bar foo lorem

#### Alias definition ending in blank causes the next word to undergo alias substitution
alias echo='echo '
alias world='world peace'
echo world
## stdout: world peace

#### Only barewords are eligible for alias expansion
alias echo='echo aliased'
'echo' line1
\echo line2
x=echo
$x line3
`printf echo` line4
## STDOUT:
line1
line2
line3
line4
## END
