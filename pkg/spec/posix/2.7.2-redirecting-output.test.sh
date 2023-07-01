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

#### Output redirection fails if noclobber is in effect and file exists
echo 'v1 content' > file
set -o noclobber
echo v2 > file
## status: [1, 127]
## stderr-regexp: .+

#### >| always overwrites
echo 'v1 content' > file
set -o noclobber
echo v2 >| file
cat file
## stdout: v2
