#### Function definition
f() {
    echo in f
}
f
## stdout: in f

#### Function name may not be that of a special builtin
break() {
    echo new break
}
echo should not get here
## status: [1, 127]
## stderr-regexp: .+

#### Function definition with subshell group body
f() (
    echo in f
)
f
## stdout: in f

#### Function definition with control structure body
x=0
f() while test $x -lt 3; do
    echo $x
    : $(( x += 1 ))
done
f
## STDOUT:
0
1
2
## END

#### Words in function body are expanded when function is invoked
x=old
f() {
    echo $x
}
x=new
f
## stdout: new

#### Fatal errors when expanding words in function body are fatal
f() {
    : $(( 1 /*/ 2 ))
}
f
echo should not get here
## status: [1, 127]
## stdout-json: ""
## stderr-regexp: .+

#### Redirections after function body happen when function is invoked
f() {
    echo $1
} >> output
f foo
f bar
cat output
## STDOUT:
foo
bar
## END

#### Fatal errors when applying redirections are fatal
f() {
    :
} >> $(( 1 /*/ 2 ))
echo should get here
f
echo should not get here
## status: [1, 127]
## stdout: should get here
## stderr-regexp: .+

#### $# and positional parameters reflect function arguments while function is executed
f() {
    echo $# $1 $2
}
f lorem ipsum
echo $# $1 $2 $3
## argv-json: ["/bin/sh", "foo", "bar", "foobar"]
## STDOUT:
2 lorem ipsum
3 foo bar foobar
## END

#### $0 is unchanged while function is executed
f() {
    echo $0
}
f lorem ipsum
## argv-json: ["/bin/sh", "foo", "bar", "foobar"]
## stdout: /bin/sh

# Effect of return is tested in its own test file.

#### Status of a successful function declaration is 0
f() { :; }
echo $?
## stdout: 0

#### Status of a failed function declaration is greater than 0
break() { :; }
## status: [1, 127]

#### Status of a function invocation is that of the last executed command
f() {
    $1
}
f true
echo $?
f false
echo $?
## STDOUT:
0
1
## END
