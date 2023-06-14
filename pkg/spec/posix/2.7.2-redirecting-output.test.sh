#### Redirecting standard output
echo content > file
cat file
## stdout: content

#### Redirecting a different FD
echo3() {
    echo "$@" >&3
}
echo3 content 3> file
cat file
## stdout: content

#### Truncating existing file
echo 'v1 content' > file
echo v2 > file
cat file
## stdout: v2

# TODO: Test "set -C"
# TODO: Test ">|"
