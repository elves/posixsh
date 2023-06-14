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

# TODO: Test "set -f"

# The relevant section in POSIX for the following tests is 2.13 "Pattern Matching
# Notation".

#### Generated filenames are sorted
touch ksh bash ash zsh dash
printf ': %s\n' *
## STDOUT:
: ash
: bash
: dash
: ksh
: zsh
## END

#### Matching one character with ?
touch foo bar baz barr
printf ': %s\n' ba? | sort
## STDOUT:
: bar
: baz
## END

#### Matching multiple characters with *
touch foo bar baz barr
printf ': %s\n' ba*
## STDOUT:
: bar
: barr
: baz
## END

#### Character set with [set]
touch foo bar bas bay baz
printf ': %s\n' ba[rst]
## STDOUT:
: bar
: bas
## END

#### Negated character set with [!set]
touch foo bar bas bay baz
printf ': %s\n' ba[!rst]
## STDOUT:
: bay
: baz
## END

# TODO: Test character range [a-z] and [!a-z] when implemented

# TODO: Test that [ is literal if not matched by ] before /
# 
# touch ax
# echo a[x
# echo a[x/]

#### [ is literal if not matched by ] before /
touch ax
echo a[x
# TODO: Also test this when it works: echo a[x/]
## STDOUT:
a[x
## END

#### Slashes must be matched explicitly
mkdir d
touch foo bar d/foo d/bar
printf ': %s\n' *
printf '* %s\n' d/*
## STDOUT:
: bar
: d
: foo
* d/bar
* d/foo
## END

#### Leading dots must be matched explicitly
mkdir d
touch d/.foo d/foo
printf ': %s\n' d/*
printf '* %s\n' d/[!a]*
printf '! %s\n' d/.*
## STDOUT:
: d/foo
* d/foo
! d/.foo
## END
