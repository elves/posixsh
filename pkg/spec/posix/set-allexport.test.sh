#### allexport causes variables to be exported when set by assignment
set -o allexport
foo=value
sh -c 'echo foo=$foo'
## stdout: foo=value

#### allexport causes variables to be exported when set by arithmetic expression
set -o allexport
: $(( foo = 42 ))
sh -c 'echo foo=$foo'
## stdout: foo=42

#### allexport causes variables to be exported when set by variable expansion ${name=value}
set -o allexport
: ${foo=value}
sh -c 'echo foo=$foo'
## stdout: foo=value

#### allexport causes variables to be exported when set by the for command
set -o allexport
for foo in value; do
    sh -c 'echo foo=$foo'
done
## stdout: foo=value

# TODO: #### allexport causes variables to be exported when set by the getopt command

#### allexport causes variables to be exported when set by the read command
set -o allexport
echo value | read foo
sh -c 'echo foo=$foo'
## stdout: foo=value

#### allexport doesn't export previously set variables
before=value
set -o allexport
sh -c 'echo $before'
## stdout:

#### Variables exported during allexport remain exported
set -o allexport
foo=value
set +o allexport
sh -c 'echo foo=$foo'
## stdout: foo=value

#### Variables that get exported in temporary assignment are unexported afterwards
foo=old
set -o allexport
f() {
    sh -c 'echo foo=$foo'
}
foo=new f
sh -c 'echo foo=$foo'
## STDOUT:
foo=new
foo=
## END

#### set -a is equivalent to set -o allexport
set -a
foo=value
sh -c 'echo foo=$foo'
## stdout: foo=value
