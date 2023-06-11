#### Special parameter $* with word splitting
IFS=/:
# Empty fields are removed, and non-empty ones undergo IFS splitting
printf ': %s\n' $*
## argv-json: ["/bin/sh", "foo/a", "", "bar"]
## STDOUT:
: foo
: a
: bar
## END

#### Special parameter $* without word splitting, non-empty IFS
IFS=/:
# Joined with first character in IFS
printf '%s\n' "$*"
## argv-json: ["/bin/sh", "foo/a", "", "bar"]
## stdout: foo/a//bar

#### Special parameter $* without word splitting, unset IFS
# Joined with spaces
printf '%s\n' "$*"
## argv-json: ["/bin/sh", "foo/a", "", "bar"]
## stdout: foo/a  bar

#### Special parameter $* without word splitting, null IFS
IFS=
# Joined with empty strings
printf '%s\n' "$*"
## argv-json: ["/bin/sh", "foo/a", "", "bar"]
## stdout: foo/abar

#### Special parameter $#
echo $#
## argv-json: ["/bin/sh", "foo", "bar"]
## STDOUT:
2
## END

#### Special parameter $0
echo $0
## argv-json: ["/bin/sh"]
## STDOUT:
/bin/sh
## END

# TODO: Test other special parameters.
