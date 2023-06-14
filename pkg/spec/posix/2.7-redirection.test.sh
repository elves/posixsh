#### Redirection with explicit FD
f() {
    echo stdout
    echo stderr >&2
}
f >stdout 2>stderr
cat stderr
## stdout: stderr

#### Escaped FD is not part of redirection
echo \2>a
cat a
## stdout: 2

#### Escaped redirection operator results in a normal word
echo 2\>a
## stdout: 2>a

# TODO: Test tilde expansion in filename

#### Filename is subject to parameter expansion
x=file
echo content > $x
cat file
## stdout: content

#### Filename is subject to command substitution
echo content > $(echo file)
cat file
## stdout: content

#### Filename is subject to arithmetic expansion
echo content > $(( 6 * 7 ))
cat 42
## stdout: content

#### Filename is subject to quote removal
echo content > "file"
cat file
## stdout: content

#### Filename is not subject to field splitting
x='file name'
echo content > $x
cat 'file name'
## stdout: content

#### Filename is not subject to pathname expansion in non-interactive shell
touch foo
echo content > *
cat '*'
## stdout: content

#### Multiple redirections are evaluated from beginning to end
echo content 2>file >&2
cat file
## stdout: content

# TODO: Test failure to open or create file
