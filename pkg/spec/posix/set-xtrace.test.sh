#### set -o xtrace causes simple commands to be printed after expansion
x=foo
set -o xtrace
echo $x `echo bar` $(( 7 * 6 ))
for x in lorem ipsum; do
    echo $x
done
hello() {
    echo Hello $1
}
hello world
## STDERR:
+ echo bar
+ echo foo bar 42
+ echo lorem
+ echo ipsum
+ hello world
+ echo Hello world
## END
## STDOUT:
foo bar 42
lorem
ipsum
Hello world
## END

#### set -x is equivalent to set -o xtrace
x=foo
set -x
echo $x
## stdout: foo
## stderr: + echo foo
