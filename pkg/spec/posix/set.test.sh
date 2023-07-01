# Effect of various options are tested in their respective files, like
# set-allexport.test.sh.

#### set lists variables when given no arguments
foo=value
set
## stdout-regexp: (?m).*^foo=value$.*

#### set sets positional arguments when given arguments
set foo bar
echo $# $1 $2
## stdout: 2 foo bar

#### "set --" unsets all positional arguments
set foo bar
set --
echo $#
## stdout: 0

# TODO: Test set -o and set +o
