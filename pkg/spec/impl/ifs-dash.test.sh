#### IFS with dashes
# We compile IFS delimiters into a regular expression. A previous implementation
# did not escape "-" properly inside brackets.
IFS=:-:
x=foo-bar
printf ': %s\n' $x
## STDOUT:
: foo
: bar
## END
