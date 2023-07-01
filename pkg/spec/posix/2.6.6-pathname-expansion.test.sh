#### Pathname expansion from unquoted wildcard
touch foo bar
printf ': %s\n' *
## STDOUT:
: bar
: foo
## END

#### Pathname expansion from unquoted variable expansion
touch foo bar
x='*'
printf ': %s\n' $x
## STDOUT:
: bar
: foo
## END

#### Pathname expansion from unquoted command substitution
touch foo bar
printf ': %s\n' $(echo '*')
## STDOUT:
: bar
: foo
## END

#### Pathname expansion is suppressed by the noglob option
touch foo bar
set -o noglob
x='*'
printf ': %s\n' * $x $(echo '*')
## STDOUT:
: *
: *
: *
## END

# More tests for the pattern syntax are found in tests for 2.13 "Pattern
# matching notation".
