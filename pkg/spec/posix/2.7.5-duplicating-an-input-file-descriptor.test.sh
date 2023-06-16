#### Duplicating standard input from another FD
echo content > file
cat 3<file <&3
## stdout: content

#### Duplicating input FD from another FD
echo content > file
cat3() {
    cat <&3
}
cat3 4<file 3<&4
## stdout: content

# TODO: Test effect of closing FD
