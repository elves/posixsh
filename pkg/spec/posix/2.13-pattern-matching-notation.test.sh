#### Matching one character with ?
touch foo bar baz barr
printf ': %s\n' ba?

x=foobar y=foobas z=foobarr
printf '* %s\n' ${x%ba?} ${y%ba?} ${z%ba?}

case foobar in
foob?) echo 'foob?' ;;
fooba?) echo 'fooba?' ;;
esac
## STDOUT:
: bar
: baz
* foo
* foo
* foobarr
fooba?
## END

#### Matching multiple characters with *
touch foo bar baz barr
printf ': %s\n' ba*

x=foobar y=foobas z=foobarr
printf '* %s\n' ${x%b*r} ${y%b*r} ${z%b*r}

case foobar in
foo*z) echo 'foo*z' ;;
foo*r) echo 'foo*r' ;;
esac
## STDOUT:
: bar
: barr
: baz
* foo
* foobas
* foo
foo*r
## END

#### Character set with [set]
touch foo bar bas bay baz
printf ': %s\n' ba[rst]

x=foobar y=foobas z=foobaz
printf '* %s\n' ${x%ba[rst]} ${y%ba[rst]} ${z%ba[rst]}

case foobar in
fooba[xyz]) echo 'fooba[xyz]' ;;
fooba[rst]) echo 'fooba[rst]' ;;
esac
## STDOUT:
: bar
: bas
* foo
* foo
* foobaz
fooba[rst]
## END

#### Negated character set with [!set]
touch foo bar bas bay baz
printf ': %s\n' ba[!rst]

x=foobar y=foobas z=foobaz
printf '* %s\n' ${x%ba[!rst]} ${y%ba[!rst]} ${z%ba[!rst]}

case foobar in
fooba[!rst]) echo 'fooba[!rst]' ;;
fooba[!xyz]) echo 'fooba[!xyz]' ;;
esac
## STDOUT:
: bay
: baz
* foobar
* foobas
* foo
fooba[!xyz]
## END

#### Character range with [a-z]
touch bab bac bar baz
printf ': %s\n' ba[a-g]

x=foobab y=foobac z=foobaz
printf '* %s\n' ${x%ba[a-g]} ${y%ba[a-g]} ${z%ba[a-g]}

case foobar in
fooba[a-g]) echo 'fooba[a-g]' ;;
fooba[h-z]) echo 'fooba[h-z]' ;;
esac
## STDOUT:
: bab
: bac
* foo
* foo
* foobaz
foo[h-z]
## END

#### ASCII character class with [[:class]]
touch bar bas ba0 ba_
printf ': %s\n' ba[[:alpha:]]

x=foobar y=foobas z=fooba0
printf '* %s\n' ${x%ba[[:alpha]]} ${y%ba[[:alpha]]} ${z%ba[[:alpha]]}

case foobar in
fooba[[:digit]]) echo 'fooba[[:digit]]' ;;
fooba[[:alpha]]) echo 'fooba[[:alpha]]' ;;
esac
## STDOUT:
: bar
: baz
* foo
* foo
* fooba0
fooba[[:alpha:]]
## END

#### Unmatched [ is literal
touch bar
printf ': %s\n' ba[

x=foobar y=foobas z=fooba[
printf '* %s\n' ${x%ba[} ${y%ba[} ${z%ba[}

case 'fooba[' in
fooba[) echo 'fooba[' ;;
esac
## STDOUT:
: ba[
* foobar
* foobas
* foo
fooba[
## END

# Tests below are for 2.13.3 "Pattern used for filename expansion".

#### Slashes must be matched explicitly
mkdir d
touch foo bar d/foo d/bar
printf ': %s\n' * # doesn't match any file in d/
printf '* %s\n' d/*
## STDOUT:
: bar
: d
: foo
* d/bar
* d/foo
## END

#### Pattern is split by slashes before pairing of brackets
touch ax
echo a[x/] # doesn't match ax because pattern is split to a[x and ] first
## STDOUT:
a[x/]
## END

#### Leading dots must be matched explicitly
mkdir d
touch d/.foo d/foo
printf '1 %s\n' d/*     # Doesn't match d/.foo
printf '2 %s\n' d/?*o   # Doesn't match d/.foo
printf '3 %s\n' d/[!a]* # Doesn't match d/.foo
printf '4 %s\n' d/.*
## STDOUT:
1 d/foo
2 d/foo
3 d/foo
4 d/.foo
## END

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

#### Pattern with no match is literal text
touch bar
printf ': %s\n' foo*
printf '* %s\n' ba[x-z]
## STDOUT:
: foo*
* ba[x-z]
## END
