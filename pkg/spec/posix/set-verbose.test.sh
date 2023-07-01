#### set -o verbose causes all top-level commands to be printed to stderr
set -o verbose
x=value
echo ": $x"
for y in foo bar; do
    echo ": $y"
done
f() {
    echo ": in f"
}
f
## STDERR:
x=value
echo ": $x"
for y in foo bar; do
    echo ": $y"
done
f() {
    echo ": in f"
}
f
## END
## STDOUT:
: value
: foo
: bar
: in f
## END

#### set -v is equivalent to set -o verbose
set -v
echo foo
## stderr: echo foo
## stdout: foo
