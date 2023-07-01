#### Special parameter $@ with word splitting
IFS=/:
# Empty fields are removed, and non-empty ones undergo IFS splitting
printf ': %s\n' $@
## argv-json: ["/bin/sh", "foo/a", "", "bar"]
## STDOUT:
: foo
: a
: bar
## END

#### Special parameter $@ in double quotes
printf ': %s\n' prefix-"$@"-suffix
## argv-json: ["/bin/sh", "foo/a", "", "bar"]
## STDOUT:
: prefix-foo/a
: 
: bar-suffix
## END

#### Special parameter $@ in other environments suppressing word splitting
IFS=/:
x=$@
printf '%s\n' "$x"
## argv-json: ["/bin/sh", "foo/a", "", "bar"]
## stdout: foo/a//bar

# Other possible values of IFS are tested in the test cases against $*, as $*
# and $@ share the same code path those environments.

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

#### Special parameter $?
false
echo $?
true
echo $?
## STDOUT:
1
0
## END

#### Special parameter $-
set +eC
echo $-
set -eC
echo $-
## stdout-regexp: [^eC]*\n.*(e.*C|C.*e).*

#### Special parameter $0
echo $0
## argv-json: ["/bin/sh"]
## STDOUT:
/bin/sh
## END

# TODO: Test $!
