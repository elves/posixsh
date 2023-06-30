#### shift shifts arguments by 1 by default
set -- a b c
echo $# $1 $2 $3
shift
echo $# $1 $2
## STDOUT:
3 a b c
2 b c
## END

#### shift shifts arguments by arbitrary offset
set -- a b c
shift 2
echo $# $1
## stdout: 1 c

#### shift shifts arguments by number of arguments
set -- a b c
shift 3
echo $#
## stdout: 0

#### Argument larger than $# is an error
set -- a b c
shift 4
# POSIX doesn't specify whether the error should be fatal, so we don't test
# that.
## status: [1, 127]
## stderr-regexp: .+
