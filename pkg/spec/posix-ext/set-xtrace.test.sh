#### xtrace prints assignments
set -o xtrace
x=foo
y=bar echo $x
## stdout: foo
## STDERR:
+ x=foo
+ y=bar echo foo
## END

#### xtrace doesn't print redirections
set -o xtrace
echo foo > /dev/null
## stderr: + echo foo
