# Notes on the expansion order tests
# 
# POSIX divides word expansions into 3 steps:
#
# 1. Tilde expansion, parameter expansion, command substitution and arithmetic
#    expansion
# 2. Field splitting
# 3. Pathname expansion
#
# To test the order of expansions, we look at each type of expansion in step 1
# in turn, and test that
#
# - Its results are not subject to other expansions in step 1
# - Its results are subject to step 2 and 3 where applicable
#
# We also test that field splitting happens before pathname expansion. A lot of
# other tests rely on this assumption, but we include it for completeness.
#
# All these tests, when combined, establish the order of the expansions.

#### Expansion order: tilde expansion vs other expansions
# Note: Tilde expansion results are considered quoted and is not subject to
# field splitting or pathname expansion.
x=value
HOME='$x $(echo) $((1+2))  *'
printf ': %s\n' ~
## stdout: : $x $(echo) $((1+2))  *

#### Expansion order: parameter expansion vs other expansions
touch file1 file2
x='~ $(echo) $((1+2)) *'
printf ': %s\n' $x
## STDOUT:
: ~
: $(echo)
: $((1+2))
: file1
: file2
## END

#### Expansion order: command substitution vs other expansions
touch file1 file2
printf ': %s\n' $(echo '~ $x $((1+2)) *')
## STDOUT:
: ~
: $x
: $((1+2))
: file1
: file2
## END

#### Expansion order: arithmetic expansion vs other expansions
# The result of arithmetic expansion is always a number, so it can't be subject
# to another of the other expansions in step 1. As a result, we can't actually
# tell if arithmetic expansion is done as an earlier step before other
# expansions. Furthermore, the result of arithmetic expansion can't be subject
# to pathname expansion.
#
# So we only test that the result of arithmetic expansion is subject to field
# splitting; this is duplicate with a test in 2.6.5-field-splitting.test.sh,
# but we include it here for completeness.
IFS=0
printf ': %s\n' $(( 99 + 2 ))
## STDOUT:
: 1
: 1
## END

#### Expansion order: field splitting vs pathname expansion
touch 'a b' bar foo
x='a *'
printf ': %s\n' $x
# If pathname generation happens before field splitting, $x should expand to
# exactly one filename, 'a b'.
## STDOUT:
: a
: a b
: bar
: foo
## END

#### Null words are deleted
x=''
printf ': %s\n' $x $(echo) $x$(echo) foo
## STDOUT:
: foo
## END

#### Null words are deleted even when IFS is empty
IFS=''
x=''
printf ': %s\n' $x $(echo) $x$(echo) foo
## STDOUT:
: foo
## END

#### Non-null words expanding to a single null word is deleted
x='   '
printf ': %s\n' $x foo
## STDOUT
: foo
## END

#### Single-quoted or partially single-quoted null words are not deleted
x=''
printf ': %s\n' '' ''$x $x'' $(echo)'' ''$(echo) foo
## STDOUT:
: 
: 
: 
: 
: 
: foo
## END

#### Double-quoted or partially double-quoted null words are not deleted
x=''
printf ': %s\n' "" ""$x $x"" ""$(echo) $(echo)"" foo
## STDOUT:
: 
: 
: 
: 
: 
: foo
## END
