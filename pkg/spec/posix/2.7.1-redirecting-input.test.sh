#### Redirecting standard input
echo content > file
cat < file
## stdout: content

#### Redirecting a different FD
cat3() {
    cat <&3
}
echo content > file
cat3 3< file
## stdout: content
